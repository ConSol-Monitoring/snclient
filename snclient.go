package snclient

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	deadlock "github.com/sasha-s/go-deadlock"
	daemon "github.com/sevlyar/go-daemon"
)

const (
	// NAME contains the snclient full official name.
	NAME = "SNClient+"

	// VERSION contains the actual snclient version.
	VERSION = "0.1"

	// ExitCodeOK is used for normal exits.
	ExitCodeOK = 0

	// ExitCodeError is used for erroneous exits.
	ExitCodeError = 2

	// ExitCodeUnknown is used for unknown exits.
	ExitCodeUnknown = 3

	// BlockProfileRateInterval sets the profiling interval when started with -profile.
	BlockProfileRateInterval = 10

	// DefaulSocketTimeout sets the default timeout for tcp sockets.
	DefaulSocketTimeout = 30
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

type Agent struct {
	Config    Config               // reference to global config object
	Listeners map[string]*Listener // Listeners stores if we started a listener
	flags     struct {             // command line flags
		flagVerbose      bool
		flagVeryVerbose  bool
		flagTraceVerbose bool
		flagVersion      bool
		flagHelp         bool
		flagConfigFile   configFiles
		flagPidfile      string
		flagMemProfile   string
		flagProfile      string
		flagCPUProfile   string
		flagLogFile      string
		flagDeadlock     int
	}
	cpuProfileHandler *os.File
	Build             string
	Revision          string
	daemonMode        bool
}

func SNClient(build, revsion string) {
	snc := Agent{
		Build:     build,
		Revision:  revsion,
		Listeners: make(map[string]*Listener),
	}

	snc.setFlags()
	snc.checkFlags()
	CreateLogger(&snc)

	// reads the args, check if they are params, if so sends them to the configuration reader
	config, listeners, err := snc.readConfiguration()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		snc.CleanExit(ExitCodeError)
	}
	CreateLogger(&snc)

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
				fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
			}
		}()
	}

	snc.createPidFile()
	defer snc.deletePidFile()

	// start usr1 routine which prints stacktraces upon request
	osSignalUsrChannel := make(chan os.Signal, 1)
	setupUsrSignalChannel(osSignalUsrChannel)

	osSignalChannel := make(chan os.Signal, 1)
	signal.Notify(osSignalChannel, syscall.SIGHUP)
	signal.Notify(osSignalChannel, syscall.SIGTERM)
	signal.Notify(osSignalChannel, os.Interrupt)
	signal.Notify(osSignalChannel, syscall.SIGINT)

	snc.startAll(config, listeners)
	log.Infof("%s v%s (Build: %s), pid: %d started\n", NAME, VERSION, snc.Build, os.Getpid())

	for {
		exitState := snc.mainLoop(osSignalChannel)
		if exitState != Reload {
			// make it possible to call mainLoop() from tests without exiting the tests
			break
		}
	}

	log.Infof("snclient exited (pid %d)\n", os.Getpid())
}

func (snc *Agent) mainLoop(osSignalChannel chan os.Signal) MainStateType {
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
				newConfig, listeners, err := snc.readConfiguration()
				if err != nil {
					log.Errorf("reloading configuration failed: %s", err.Error())

					continue
				}

				CreateLogger(snc)
				snc.startAll(newConfig, listeners)

				return exitCode
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

func (snc *Agent) startAll(config Config, listeners map[string]*Listener) {
	// stop existing listeners
	for name, l := range listeners {
		l.Stop()
		delete(snc.Listeners, name)
	}

	snc.Config = config
	snc.Listeners = listeners

	for name := range snc.Listeners {
		snc.startListener(name)
	}
}

func (snc *Agent) readConfiguration() (Config, map[string]*Listener, error) {
	config := NewConfig()

	if len(snc.flags.flagConfigFile) == 0 {
		snc.flags.flagConfigFile = append(snc.flags.flagConfigFile, "snclient.ini")
	}

	for _, path := range snc.flags.flagConfigFile {
		err := config.ReadSettingsFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("reading settings failed: %s", err.Error())
		}
	}

	// set paths
	pathSection := config.Section("/paths")
	exe, ok, err := pathSection.GetString("exe-path")
	switch {
	case err != nil:
		return nil, nil, fmt.Errorf("reading exe-path settings failed: %s", err.Error())
	case ok && exe != "":
		log.Warnf("exe-path should not be set manually")

		fallthrough
	default:
		executable, err := os.Executable()
		if err != nil {
			return nil, nil, fmt.Errorf("could not detect path to executable: %s", err.Error())
		}

		executable, err = filepath.Abs(executable)
		if err != nil {
			return nil, nil, fmt.Errorf("could not detect abs path to executable: %s", err.Error())
		}

		(*pathSection)["exe-path"] = filepath.Dir(executable)
	}

	for _, key := range []string{"exe-path", "shared-path", "scripts", "certificate-path"} {
		val, ok, err := pathSection.GetString(key)
		switch {
		case err != nil:
			return nil, nil, fmt.Errorf("reading %s settings failed: %s", key, err.Error())
		case !ok || val == "":
			(*pathSection)[key] = (*pathSection)["exe-path"]
		}
	}

	for key, val := range *pathSection {
		val = config.ReplaceMacros(val)
		(*pathSection)[key] = val
	}

	// replace other sections
	for _, section := range config {
		for key, val := range section {
			val = config.ReplaceMacros(val)
			section[key] = val
		}
	}

	for key, val := range *pathSection {
		log.Tracef("conf macro: %s -> %s", key, val)
	}

	listen, err := snc.initListeners(config)
	if err != nil {
		return nil, nil, fmt.Errorf("listener initialization failed: %s", err.Error())
	}

	if len(listen) == 0 {
		return nil, nil, fmt.Errorf("no listener enabled, bailing out")
	}

	return config, listen, nil
}

func (snc *Agent) initListeners(conf Config) (map[string]*Listener, error) {
	listen := make(map[string]*Listener)

	modulesConf := conf.Section("/modules")
	for _, entry := range AvailableListeners {
		enabled, ok, err := modulesConf.GetBool(entry.ModuleKey)
		switch {
		case err != nil:
			return nil, fmt.Errorf("error in %s listener configuration: %s", entry.ModuleKey, err.Error())
		case !ok:
			continue
		case !enabled:
			continue
		}

		listenConf := conf.Section(entry.ConfigKey).Clone()
		listenConf.Merge(*conf.Section("/settings/default"))
		listenConf.Merge(entry.Init.Defaults())

		listener, err := snc.initListener(listenConf, entry.Init)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", entry.ConfigKey, err.Error())
		}

		bind := listener.BindString()
		if existing, ok := listen[bind]; ok {
			return nil, fmt.Errorf("bind address %s already in use by %s server", bind, existing.handler.Type())
		}

		listen[bind] = listener
	}

	return listen, nil
}

func (snc *Agent) cleanExit(exitCode int) {
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

func (snc *Agent) createPidFile() {
	// write the pid id if file path is defined
	if snc.flags.flagPidfile == "" {
		return
	}
	// check existing pid
	if snc.checkStalePidFile() {
		fmt.Fprintf(os.Stderr, "Warning: removing stale pidfile %s\n", snc.flags.flagPidfile)
	}

	err := os.WriteFile(snc.flags.flagPidfile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not write pidfile: %s\n", err.Error())
		snc.cleanExit(ExitCodeError)
	}
}

func (snc *Agent) checkStalePidFile() bool {
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

func (snc *Agent) deletePidFile() {
	if snc.flags.flagPidfile != "" {
		os.Remove(snc.flags.flagPidfile)
	}
}

// printVersion prints the version.
func (snc *Agent) printVersion() {
	fmt.Fprintf(os.Stdout, "snclient+ v%s.%s (Build: %s)\n", VERSION, snc.Revision, snc.Build)
}

func (snc *Agent) printUsage(full bool) {
	usageOutput := os.Stdout
	fmt.Fprintf(usageOutput, "Usage: snclient [OPTION]...\n")
	fmt.Fprintf(usageOutput, "\n")
	fmt.Fprintf(usageOutput, "snclient+ agent runs checks on various platforms.\n")
	fmt.Fprintf(usageOutput, "\n")
	fmt.Fprintf(usageOutput, "Basic Settings:\n")
	fmt.Fprintf(usageOutput, "       --daemon                                     \n")
	fmt.Fprintf(usageOutput, "       --debug=<lvl>                                \n")
	fmt.Fprintf(usageOutput, "       --logmode=<automatic|stdout|syslog|file>     \n")
	fmt.Fprintf(usageOutput, "       --logfile=<path>                             \n")
	fmt.Fprintf(usageOutput, "       --help|-h                                    \n")
	fmt.Fprintf(usageOutput, "       --config=<configfile>                        \n")
	fmt.Fprintf(usageOutput, "\n")
	fmt.Fprintf(usageOutput, "see README for a detailed explanation of all options.\n")
	fmt.Fprintf(usageOutput, "\n")

	if full {
		flag.CommandLine.SetOutput(usageOutput)
		flag.Usage()
	}

	os.Exit(ExitCodeUnknown)
}

func (snc *Agent) setFlags() {
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
}

func (snc *Agent) checkFlags() {
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

			server := &http.Server{
				Addr:              snc.flags.flagProfile,
				ReadTimeout:       DefaulSocketTimeout * time.Second,
				ReadHeaderTimeout: DefaulSocketTimeout * time.Second,
				WriteTimeout:      DefaulSocketTimeout * time.Second,
				IdleTimeout:       DefaulSocketTimeout * time.Second,
			}

			err := server.ListenAndServe()
			if err != nil {
				log.Debugf("http.ListenAndServe finished with: %e", err)
			}
		}()
	}

	if snc.flags.flagCPUProfile != "" {
		runtime.SetBlockProfileRate(BlockProfileRateInterval)

		cpuProfileHandler, err := os.Create(snc.flags.flagCPUProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not create CPU profile: %s\n", err.Error())
			os.Exit(ExitCodeError)
		}

		if err := pprof.StartCPUProfile(cpuProfileHandler); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not start CPU profile: %s\n", err.Error())
			os.Exit(ExitCodeError)
		}

		snc.cpuProfileHandler = cpuProfileHandler
	}

	if snc.flags.flagDeadlock <= 0 {
		deadlock.Opts.Disable = true
	} else {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Duration(snc.flags.flagDeadlock) * time.Second
		deadlock.Opts.LogBuf = NewLogWriter("Error")
	}
}

func (snc *Agent) CleanExit(exitCode int) {
	snc.deletePidFile()

	if snc.flags.flagCPUProfile != "" {
		pprof.StopCPUProfile()
		snc.cpuProfileHandler.Close()
		log.Infof("cpu profile written to: %s", snc.flags.flagCPUProfile)
	}

	os.Exit(exitCode)
}

func (snc *Agent) logPanicExit() {
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

func (snc *Agent) initListener(conConf ConfigSection, handler RequestHandler) (*Listener, error) {
	listener, err := NewListener(snc, conConf, handler)
	if err != nil {
		return nil, err
	}

	err = handler.Init(snc, conConf)
	if err != nil {
		listener.Stop()

		return nil, fmt.Errorf("%s init failed on %s: %s", handler.Type(), listener.BindString(), err.Error())
	}

	return listener, nil
}

func (snc *Agent) startListener(name string) {
	listener, ok := snc.Listeners[name]
	if !ok {
		log.Errorf("no listener with name: %s", name)

		return
	}

	err := snc.Listeners[name].Start()
	if err != nil {
		log.Errorf("failed to start %s listener:  %s", err.Error())
		listener.Stop()
		delete(snc.Listeners, name)
	}
}

func (snc *Agent) RunCheck(name string, args []string) *CheckResult {
	check, ok := AvailableChecks[name]
	if !ok {
		res := CheckResult{
			State:  CheckExitUnknown,
			Output: "No such check",
		}

		return &res
	}

	res, err := check.Handler.Check(args)
	if err != nil {
		res := CheckResult{
			State:  CheckExitUnknown,
			Output: err.Error(),
		}

		return &res
	}

	return res
}
