package snclient

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	deadlock "github.com/sasha-s/go-deadlock"
	daemon "github.com/sevlyar/go-daemon"
	log "github.com/sirupsen/logrus"
)

const (
	// VERSION contains the actual lmd version.
	VERSION = "0.0.1"

	// ExitCodeOK is used for normal exits.
	ExitCodeOK = 0

	// ExitCodeError is used for erroneous exits.
	ExitCodeError = 2

	// ExitCodeUnknown is used for unknown exits.
	ExitCodeUnknown = 3

	// BlockProfileRateInterval sets the profiling interval when started with -profile.
	BlockProfileRateInterval = 10
)

// MainStateType is used to set different states of the main loop.
type MainStateType int

const (
	// Reload flag if used after a sighup.
	Reload MainStateType = iota

	// Shutdown is used when sigint received.
	Shutdown

	// ShutdownGraceFully is used when sigterm received.
	ShutdownGraceFully

	// Resume is used when signal does not change main state.
	Resume
)

// https://github.com/golang/go/issues/8005#issuecomment-190753527
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type arrayFlags struct {
	list []string
}

func (i *arrayFlags) String() string {
	return strings.Join(i.list, ", ")
}

func (i *arrayFlags) Set(value string) error {
	i.list = append(i.list, value)

	return nil
}

type SNClientInstance struct {
	Config    Config               // reference to global config object
	Listeners map[string]*Listener // Listeners stores if we started a listener
	flags     struct {             // command line flags
		flagCmd          string
		flagVerbose      bool
		flagVeryVerbose  bool
		flagTraceVerbose bool
		flagConfigFile   configFiles
		flagCfgOption    arrayFlags
		flagPidfile      string
		flagMemProfile   string
		flagVersion      bool
		flagHelp         bool
		flagProfile      string
		flagCPUProfile   string
		flagLogFile      string
		flagDeadlock     int
	}
	cpuProfileHandler *os.File
	mainSignalChannel chan os.Signal
	Build             string
	daemonMode        bool
}

func SNClient(build string) {
	snc := SNClientInstance{
		Build:     build,
		Listeners: make(map[string]*Listener),
	}

	snc.setFlags()
	snc.checkFlags()

	// reads the args, check if they are params, if so sends them to the configuration reader
	config, err := snc.initConfiguration()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		snc.CleanExit(ExitCodeError)
	}

	snc.Config = config

	defer snc.logPanicExit()

	// daemonize
	if snc.daemonMode {
		ctx := &daemon.Context{}

		d, err := ctx.Reborn()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: unable to start daemon mode")
		}

		if d != nil {
			return
		}

		defer func() {
			err := ctx.Release()
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %s", err.Error())
			}
		}()
	}

	snc.createPidFile()
	defer snc.deletePidFile()

	// start usr1 routine which prints stacktraces upon request
	osSignalUsrChannel := make(chan os.Signal, 1)
	setupUsrSignalChannel(osSignalUsrChannel)

	for {
		exitState := snc.mainLoop()
		if exitState != Reload {
			// make it possible to call mainLoop() from tests without exiting the tests
			break
		}
	}
}

func (snc *SNClientInstance) mainLoop() MainStateType {
	log.Infof("snclient v%s (Build: %s), pid: %d\n", VERSION, snc.Build, os.Getpid())

	snc.startListener("Prometheus", NewHandlerPrometheus())
	snc.startListener("NRPE", NewHandlerNRPE())

	osSignalChannel := make(chan os.Signal, 1)
	signal.Notify(osSignalChannel, syscall.SIGHUP)
	signal.Notify(osSignalChannel, syscall.SIGTERM)
	signal.Notify(osSignalChannel, os.Interrupt)
	signal.Notify(osSignalChannel, syscall.SIGINT)

	// just wait till someone hits ctrl+c or we have to reload
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-ticker.C:
			continue
		case sig := <-osSignalChannel:
			exitCode := mainSignalHandler(sig, snc)
			switch exitCode {
			case Resume:
				continue
			case Reload:
				newConfig, err := snc.initConfiguration()
				if err != nil {
					log.Errorf("reloading configuration failed: %w_ %s", err, err.Error())

					continue
				}

				snc.Config = newConfig

				fallthrough
			case Shutdown, ShutdownGraceFully:
				ticker.Stop()

				for name, l := range snc.Listeners {
					l.Stop()
					delete(snc.Listeners, name)
				}

				return exitCode
			}
		}
	}
}

func (snc *SNClientInstance) initConfiguration() (Config, error) {
	config := NewConfig()

	err := config.readSettingsFile("snclient.ini") // TODO: ...
	if err != nil {
		return nil, fmt.Errorf("%s", err.Error())
	}

	// TODO: ...
	log.SetFormatter(&log.TextFormatter{
		PadLevelText:    true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	log.SetLevel(log.TraceLevel)

	return config, nil
}

func (snc *SNClientInstance) cleanExit(exitCode int) {
	snc.deletePidFile()
	os.Exit(exitCode)
}

func logThreaddump() {
	buf := make([]byte, 1<<16)

	if n := runtime.Stack(buf, true); n < len(buf) {
		buf = buf[:n]
	}

	log.Errorf("threaddump:\n%s", buf)
}

func (snc *SNClientInstance) createPidFile() {
	// write the pid id if file path is defined
	if snc.flags.flagPidfile == "" {
		return
	}
	// check existing pid
	if snc.checkStalePidFile() {
		fmt.Fprintf(os.Stderr, "Warning: removing stale pidfile %s\n", snc.flags.flagPidfile)
	}

	err := os.WriteFile(snc.flags.flagPidfile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not write pidfile: %s\n", err.Error())
		snc.cleanExit(ExitCodeError)
	}
}

func (snc *SNClientInstance) checkStalePidFile() bool {
	dat, err := os.ReadFile(snc.flags.flagPidfile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(dat)))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	if err == nil {
		fmt.Fprintf(os.Stderr, "Error: worker already running: %d\n", pid)
		snc.cleanExit(ExitCodeError)
	}

	return true
}

func (snc *SNClientInstance) deletePidFile() {
	if snc.flags.flagPidfile != "" {
		os.Remove(snc.flags.flagPidfile)
	}
}

// printVersion prints the version.
func (snc *SNClientInstance) printVersion() {
	fmt.Fprintf(os.Stdout, "snclient+ v%s (Build: %s)\n", VERSION, snc.Build)
}

func (snc *SNClientInstance) printUsage(full bool) {
	// TODO: rework
	fmt.Fprintf(os.Stdout, "Usage: snclient [OPTION]...\n")
	fmt.Fprintf(os.Stdout, "\n")
	fmt.Fprintf(os.Stdout, "snclient+ agent runs checks on various platforms.\n")
	fmt.Fprintf(os.Stdout, "\n")
	fmt.Fprintf(os.Stdout, "Basic Settings:\n")
	fmt.Fprintf(os.Stdout, "       --daemon                                     \n")
	fmt.Fprintf(os.Stdout, "       --debug=<lvl>                                \n")
	fmt.Fprintf(os.Stdout, "       --logmode=<automatic|stdout|syslog|file>     \n")
	fmt.Fprintf(os.Stdout, "       --logfile=<path>                             \n")
	fmt.Fprintf(os.Stdout, "       --help|-h                                    \n")
	fmt.Fprintf(os.Stdout, "       --config=<configfile>                        \n")
	fmt.Fprintf(os.Stdout, "\n")
	fmt.Fprintf(os.Stdout, "see README for a detailed explanation of all options.\n")
	fmt.Fprintf(os.Stdout, "\n")

	if full {
		flag.Usage()
	}

	os.Exit(ExitCodeUnknown)
}

func (snc *SNClientInstance) setFlags() {
	flag.Var(&snc.flags.flagConfigFile, "c", "set path to config file / can be used multiple times / supports globs, ex.: *.ini")
	flag.Var(&snc.flags.flagConfigFile, "config", "set path to config file / can be used multiple times / supports globs, ex.: *.ini")
	flag.StringVar(&snc.flags.flagPidfile, "pidfile", "", "set path to pidfile")
	flag.StringVar(&snc.flags.flagLogFile, "logfile", "", "override logfile from the configuration file")
	flag.BoolVar(&snc.flags.flagVerbose, "v", false, "enable verbose output")
	flag.BoolVar(&snc.flags.flagVerbose, "verbose", false, "enable verbose output")
	flag.BoolVar(&snc.flags.flagVeryVerbose, "vv", false, "enable very verbose output")
	flag.BoolVar(&snc.flags.flagTraceVerbose, "vvv", false, "enable trace output")
	flag.BoolVar(&snc.flags.flagVersion, "version", false, "print version and exit")
	flag.BoolVar(&snc.flags.flagVersion, "V", false, "print version and exit")
	flag.BoolVar(&snc.flags.flagHelp, "help", false, "print help and exit")
	flag.BoolVar(&snc.flags.flagHelp, "h", false, "print help and exit")
	flag.StringVar(&snc.flags.flagProfile, "debug-profiler", "", "start pprof profiler on this port, ex. :6060")
	flag.StringVar(&snc.flags.flagCPUProfile, "cpuprofile", "", "write cpu profile to `file`")
	flag.StringVar(&snc.flags.flagMemProfile, "memprofile", "", "write memory profile to `file`")
	flag.IntVar(&snc.flags.flagDeadlock, "debug-deadlock", 0, "enable deadlock detection with given timeout")
	flag.Var(&snc.flags.flagCfgOption, "o", "override settings, ex.: -o Listen=:3333 -o Connections=name,address")
}

func (snc *SNClientInstance) checkFlags() {
	flag.Parse()

	if snc.flags.flagVersion {
		snc.printVersion()
		os.Exit(ExitCodeOK)
	}

	if snc.flags.flagHelp {
		snc.printUsage(true)
		os.Exit(ExitCodeOK)
	}

	if snc.flags.flagProfile != "" {
		if snc.flags.flagCPUProfile != "" || snc.flags.flagMemProfile != "" {
			fmt.Fprintf(os.Stderr, "ERROR: either use -debug-profile or -cpu/memprofile, not both\n")
			os.Exit(ExitCodeError)
		}

		runtime.SetBlockProfileRate(BlockProfileRateInterval)
		runtime.SetMutexProfileFraction(BlockProfileRateInterval)

		go func() {
			// make sure we log panics properly
			defer snc.logPanicExit()

			err := http.ListenAndServe(snc.flags.flagProfile, http.DefaultServeMux)
			if err != nil {
				log.Debugf("http.ListenAndServe finished with: %e", err)
			}
		}()
	}

	if snc.flags.flagCPUProfile != "" {
		runtime.SetBlockProfileRate(BlockProfileRateInterval)

		cpuProfileHandler, err := os.Create(snc.flags.flagCPUProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not create CPU profile: %s", err.Error())
			os.Exit(ExitCodeError)
		}

		if err := pprof.StartCPUProfile(cpuProfileHandler); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not start CPU profile: %s", err.Error())
			os.Exit(ExitCodeError)
		}

		snc.cpuProfileHandler = cpuProfileHandler
	}

	if snc.flags.flagDeadlock <= 0 {
		deadlock.Opts.Disable = true
	} else {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Duration(snc.flags.flagDeadlock) * time.Second
		// deadlock.Opts.LogBuf = NewLogWriter("Error") // TODO: ...
	}
}

func (snc *SNClientInstance) CleanExit(exitCode int) {
	snc.deletePidFile()

	if snc.flags.flagCPUProfile != "" {
		pprof.StopCPUProfile()
		snc.cpuProfileHandler.Close()
		log.Infof("cpu profile written to: %s", snc.flags.flagCPUProfile)
	}

	os.Exit(exitCode)
}

func (snc *SNClientInstance) logPanicExit() {
	if r := recover(); r != nil {
		log.Errorf("********* PANIC *********")
		log.Errorf("Panic: %s", r)
		log.Errorf("**** Stack:")
		log.Errorf("%s", debug.Stack())
		log.Errorf("*************************")
		snc.deletePidFile()
		os.Exit(ExitCodeError)
	}
}

func (snc *SNClientInstance) startListener(configKey string, handler RequestHandler) {
	conConf, ok := snc.Config[configKey]
	if !ok {
		return
	}

	name := handler.Type()
	defaults := handler.Defaults()

	// apply default values.
	for key, value := range defaults {
		if _, ok := conConf[key]; !ok {
			conConf[key] = value
		}
	}

	listener, err := NewListener(snc, conConf, handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		snc.CleanExit(ExitCodeError)
	}

	snc.Listeners[name] = listener

	err = handler.Init(snc)
	if err != nil {
		log.Errorf("failed to init %s listener: %w: %s", err, err.Error())
		listener.Stop()
		delete(snc.Listeners, name)
	}

	err = snc.Listeners[name].Start()
	if err != nil {
		log.Errorf("failed to start %s listener: %w: %s", err, err.Error())
		listener.Stop()
		delete(snc.Listeners, name)
	}
}
