package snclient

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"pkg/utils"

	deadlock "github.com/sasha-s/go-deadlock"
	daemon "github.com/sevlyar/go-daemon"
	"github.com/shirou/gopsutil/v3/host"
)

const (
	// NAME contains the snclient full official name.
	NAME = "SNClient+"

	DESCRIPTION = "SNClient+ (Secure Naemon Client) is a secure general purpose" +
		" monitoring agent designed as replacement for NRPE and NSClient++."

	// VERSION contains the actual snclient version.
	VERSION = "0.02"

	// ExitCodeOK is used for normal exits.
	ExitCodeOK = 0

	// ExitCodeError is used for erroneous exits.
	ExitCodeError = 2

	// ExitCodeUnknown is used for unknown exits.
	ExitCodeUnknown = 3

	// BlockProfileRateInterval sets the profiling interval when started with -profile.
	BlockProfileRateInterval = 10

	// DefaultSocketTimeout sets the default timeout for tcp sockets.
	DefaultSocketTimeout = 30
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

var AvailableTasks []*LoadableModule
var AvailableListeners []ListenHandler

// https://github.com/golang/go/issues/8005#issuecomment-190753527
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type Agent struct {
	Config    *Config              // reference to global config object
	Listeners map[string]*Listener // Listeners stores if we started listeners
	Tasks     *ModuleSet           // Tasks stores if we started task runners
	Counter   *CounterSet          // Counter stores collected counters from tasks
	flagset   *flag.FlagSet
	flags     struct { // command line flags
		flagDaemon       bool
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
		flagLogFormat    string
		flagDeadlock     int
	}
	cpuProfileHandler *os.File
	Build             string
	Revision          string
	initSet           *AgentRunSet
	osSignalChannel   chan os.Signal
	running           atomic.Bool
}

// AgentRunSet contains the initial startup config items
type AgentRunSet struct {
	config    *Config
	listeners map[string]*Listener
	tasks     *ModuleSet
}

// NewAgent returns a new Agent object ready to be started by Run()
func NewAgent(build, revision string, args []string) *Agent {
	snc := &Agent{
		Build:     build,
		Revision:  revision,
		Listeners: make(map[string]*Listener),
		Tasks:     NewModuleSet("task"),
		Counter:   NewCounerSet(),
		Config:    NewConfig(),
	}
	snc.setFlags()
	snc.checkFlags(args)
	snc.createLogger(nil)

	// reads the args, check if they are params, if so sends them to the configuration reader
	initSet, err := snc.init()
	if err != nil {
		LogStderrf("ERROR: %s", err.Error())
		snc.CleanExit(ExitCodeError)
	}
	snc.initSet = initSet
	snc.createLogger(initSet.config)

	// daemonize
	if snc.flags.flagDaemon {
		snc.daemonize()

		return nil
	}

	snc.osSignalChannel = make(chan os.Signal, 1)

	return snc
}

// IsRunning returns true if the agent is running
func (snc *Agent) IsRunning() bool {
	return snc.running.Load()
}

// Run starts the mainloop and blocks until Stop() is called
func (snc *Agent) Run() {
	defer snc.logPanicExit()

	if snc.IsRunning() {
		log.Panicf("agent is already running")
	}

	log.Infof("%s", snc.buildStartupMsg())

	snc.createPidFile()
	defer snc.deletePidFile()

	// start usr1 routine which prints stacktraces upon request
	osSignalUsrChannel := make(chan os.Signal, 1)
	setupUsrSignalChannel(osSignalUsrChannel)

	signal.Notify(snc.osSignalChannel, syscall.SIGHUP)
	signal.Notify(snc.osSignalChannel, syscall.SIGTERM)
	signal.Notify(snc.osSignalChannel, os.Interrupt)
	signal.Notify(snc.osSignalChannel, syscall.SIGINT)

	snc.startModules(snc.initSet)
	snc.running.Store(true)

	for {
		exitState := snc.mainLoop()
		if exitState != Reload {
			// make it possible to call mainLoop() from tests without exiting the tests
			break
		}
	}

	snc.running.Store(false)
	log.Infof("snclient exited (pid %d)\n", os.Getpid())
}

// RunBackground starts the agent in the background and returns immediately
func (snc *Agent) RunBackground() {
	ctx := &daemon.Context{}

	daemonProc, err := ctx.Reborn()
	if err != nil {
		LogStderrf("ERROR: unable to start daemon mode")

		return
	}

	// parent simply exits
	if daemonProc != nil {
		os.Exit(ExitCodeOK)
	}

	defer func() {
		err := ctx.Release()
		if err != nil {
			LogStderrf("ERROR: %s", err.Error())
		}
	}()

	snc.Run()
}

func (snc *Agent) mainLoop() MainStateType {
	// just wait till someone hits ctrl+c or we have to reload
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-ticker.C:
			continue
		case sig := <-snc.osSignalChannel:
			exitCode := mainSignalHandler(sig, snc)
			switch exitCode {
			case Resume:
				continue
			case Reload:
				updateSet, err := snc.init()
				if err != nil {
					log.Errorf("reloading configuration failed: %s", err.Error())

					continue
				}

				snc.createLogger(updateSet.config)
				snc.startModules(updateSet)

				return exitCode
			case Shutdown, ShutdownGraceFully:
				ticker.Stop()
				snc.stop()

				return exitCode
			}
		}
	}
}

// StartWait calls Run() and waits till the agent is started. Returns true if start was successful
func (snc *Agent) StartWait(maxWait time.Duration) bool {
	go snc.Run()

	waitUntil := time.Now().Add(maxWait)
	for {
		if snc.IsRunning() {
			return true
		}

		if time.Now().After(waitUntil) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Stop sends the shutdown signal
func (snc *Agent) Stop() {
	snc.osSignalChannel <- os.Interrupt
}

// StopWait sends the shutdown signal and waits till the agent is stopped
func (snc *Agent) StopWait(maxWait time.Duration) bool {
	snc.Stop()

	waitUntil := time.Now().Add(maxWait)
	for {
		if !snc.IsRunning() {
			return true
		}

		if time.Now().After(waitUntil) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (snc *Agent) stop() {
	snc.Tasks.StopRemove()

	for name, l := range snc.Listeners {
		l.Stop()
		delete(snc.Listeners, name)
	}
}

func (snc *Agent) startModules(initSet *AgentRunSet) {
	// stop existing tasks
	snc.Tasks.StopRemove()

	// stop existing listeners
	for name, l := range initSet.listeners {
		l.Stop()
		delete(snc.Listeners, name)
	}

	snc.Config = initSet.config
	snc.Listeners = initSet.listeners
	snc.Tasks = initSet.tasks

	snc.Tasks.Start()
	for name := range snc.Listeners {
		snc.startListener(name)
	}

	snc.initSet = initSet
}

func (snc *Agent) init() (*AgentRunSet, error) {
	var files configFiles
	files = snc.flags.flagConfigFile

	defaultLocations := []string{"./snclient.ini", "/etc/snclient/snclient.ini"}
	execPath, err := utils.GetExecutablePath()
	if err == nil {
		defaultLocations = append(defaultLocations, path.Join(execPath, "snclient.ini"))
	}

	// no config supplied, check default locations, first match wins
	if len(files) == 0 {
		for _, f := range defaultLocations {
			_, err := os.Stat(f)
			if os.IsNotExist(err) {
				continue
			}
			files = append(files, f)

			break
		}
	}

	// still empty
	if len(files) == 0 {
		return nil, fmt.Errorf("no config file supplied (--config=..) and no config file found in default locations (%s)",
			strings.Join(defaultLocations, ", "))
	}

	initSet, err := snc.readConfiguration(files)
	if err != nil {
		return nil, err
	}

	return initSet, nil
}

func (snc *Agent) readConfiguration(file []string) (*AgentRunSet, error) {
	config := NewConfig()
	for _, path := range file {
		err := config.ReadINI(path)
		if err != nil {
			return nil, fmt.Errorf("reading settings failed: %s", err.Error())
		}
	}

	// apply defaults
	for sectionName, defaults := range DefaultConfig {
		section := config.Section(sectionName)
		section.data.Merge(*defaults)
	}

	// set paths
	pathSection := config.Section("/paths")
	exe, ok := pathSection.GetString("exe-path")
	switch {
	case ok && exe != "":
		log.Warnf("exe-path should not be set manually")

		fallthrough
	default:
		execPath, err := utils.GetExecutablePath()
		if err != nil {
			return nil, fmt.Errorf("could not detect path to executable: %s", err.Error())
		}

		pathSection.Set("exe-path", execPath)
	}

	for _, key := range []string{"exe-path", "shared-path", "scripts", "certificate-path"} {
		val, ok := pathSection.GetString(key)
		if !ok || val == "" {
			pathSection.Set(key, pathSection.data["exe-path"])
		}
	}

	for key, val := range pathSection.data {
		val = config.replaceMacros(val)
		pathSection.Set(key, val)
	}

	// replace other sections
	for _, section := range config.sections {
		for key, val := range section.data {
			val = config.replaceMacros(val)
			section.data[key] = val
		}
	}

	for key, val := range pathSection.data {
		log.Tracef("conf macro: %s -> %s", key, val)
	}

	tasks, err := snc.initModules("tasks", AvailableTasks, config)
	if err != nil {
		log.Errorf("task initialization failed: %s", err.Error())
	}

	listen, err := snc.initListeners(config)
	if err != nil {
		log.Errorf("listener initialization failed: %s", err.Error())
	}

	if len(listen) == 0 {
		log.Warnf("no listener enabled")
	}

	return &AgentRunSet{
		config:    config,
		listeners: listen,
		tasks:     tasks,
	}, nil
}

func (snc *Agent) initModules(name string, loadable []*LoadableModule, conf *Config) (*ModuleSet, error) {
	modules := NewModuleSet(name)

	modulesConf := conf.Section("/modules")
	for _, entry := range loadable {
		enabled, ok, err := modulesConf.GetBool(entry.ModuleKey)
		switch {
		case err != nil:
			return nil, fmt.Errorf("error in %s /modules configuration: %s", entry.Name(), err.Error())
		case !ok:
			log.Tracef("%s %s is disabled by default config. skipping...", name, entry.Name())

			continue
		case !enabled:
			log.Tracef("%s %s is disabled by config. skipping...", name, entry.Name())

			continue
		}

		mod, err := entry.Init(snc, conf)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", entry.ConfigKey, err.Error())
		}

		err = modules.Add(entry.Name(), mod)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", entry.ConfigKey, err.Error())
		}
	}

	return modules, nil
}

func (snc *Agent) initListeners(conf *Config) (map[string]*Listener, error) {
	listen := make(map[string]*Listener)

	modulesConf := conf.Section("/modules")
	for _, entry := range AvailableListeners {
		enabled, ok, err := modulesConf.GetBool(entry.ModuleKey)
		switch {
		case err != nil:
			return nil, fmt.Errorf("error in %s /modules configuration: %s", entry.ModuleKey, err.Error())
		case !ok:
			continue
		case !enabled:
			continue
		}

		listenConf := conf.Section(entry.ConfigKey).Clone()
		listenConf.data.Merge(conf.Section("/settings/default").data)
		listenConf.data.Merge(entry.Init.Defaults())

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

func (snc *Agent) createPidFile() {
	// write the pid id if file path is defined
	if snc.flags.flagPidfile == "" {
		return
	}
	// check existing pid
	if snc.checkStalePidFile() {
		LogStderrf("WARNING: removing stale pidfile %s", snc.flags.flagPidfile)
	}

	err := os.WriteFile(snc.flags.flagPidfile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600)
	if err != nil {
		LogStderrf("ERROR: Could not write pidfile: %s", err.Error())
		snc.CleanExit(ExitCodeError)
	}
}

func (snc *Agent) checkStalePidFile() bool {
	pid, err := utils.ReadPid(snc.flags.flagPidfile)
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	if err == nil {
		LogStderrf("ERROR: worker already running: %d", pid)
		snc.CleanExit(ExitCodeError)
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
	fmt.Fprintf(os.Stdout, "%s v%s.%s (Build: %s)\n", NAME, VERSION, snc.Revision, snc.Build)
}

func (snc *Agent) printUsage(full bool) {
	usageOutput := os.Stdout
	fmt.Fprintf(usageOutput, "Usage: snclient [OPTION]...\n")
	fmt.Fprintf(usageOutput, "\n")
	fmt.Fprintf(usageOutput, "snclient+ agent runs checks and provides metrics.\n")
	fmt.Fprintf(usageOutput, "\n")
	fmt.Fprintf(usageOutput, "Basic Settings:\n")
	fmt.Fprintf(usageOutput, "    --daemon                                     \n")
	fmt.Fprintf(usageOutput, "    --config=<configfile>                        \n")
	fmt.Fprintf(usageOutput, "    --help|-h                                    \n")
	fmt.Fprintf(usageOutput, "\n")

	if full {
		snc.flagset.SetOutput(usageOutput)
		snc.flagset.PrintDefaults()
	}

	fmt.Fprintf(usageOutput, "\n")
	fmt.Fprintf(usageOutput, "see README for a detailed explanation of all options.\n")
	fmt.Fprintf(usageOutput, "\n")

	os.Exit(ExitCodeUnknown)
}

func (snc *Agent) setFlags() {
	flags := &flag.FlagSet{}
	snc.flagset = flags
	flags.Var(&snc.flags.flagConfigFile, "c", "set path to config file / can be used multiple times / supports globs, ex.: *.ini")
	flags.Var(&snc.flags.flagConfigFile, "config", "set path to config file / can be used multiple times / supports globs, ex.: *.ini")
	flags.BoolVar(&snc.flags.flagDaemon, "d", false, "start snclient as daemon in background")
	flags.BoolVar(&snc.flags.flagDaemon, "daemon", false, "start snclient as daemon in background")
	flags.StringVar(&snc.flags.flagPidfile, "pidfile", "", "set path to pidfile")
	flags.StringVar(&snc.flags.flagLogFile, "logfile", "", "override logfile from the configuration file")
	flags.StringVar(&snc.flags.flagLogFormat, "logformat", "", "override logformat, see https://pkg.go.dev/github.com/kdar/factorlog")
	flags.BoolVar(&snc.flags.flagVerbose, "v", false, "enable verbose output")
	flags.BoolVar(&snc.flags.flagVerbose, "verbose", false, "enable verbose output")
	flags.BoolVar(&snc.flags.flagVeryVerbose, "vv", false, "enable very verbose output")
	flags.BoolVar(&snc.flags.flagTraceVerbose, "vvv", false, "enable trace output")
	flags.BoolVar(&snc.flags.flagVersion, "version", false, "print version and exit")
	flags.BoolVar(&snc.flags.flagVersion, "V", false, "print version and exit")
	flags.BoolVar(&snc.flags.flagHelp, "help", false, "print help and exit")
	flags.BoolVar(&snc.flags.flagHelp, "h", false, "print help and exit")
	flags.StringVar(&snc.flags.flagProfile, "debug-profiler", "", "start pprof profiler on this port, ex. :6060")
	flags.StringVar(&snc.flags.flagCPUProfile, "cpuprofile", "", "write cpu profile to `file`")
	flags.StringVar(&snc.flags.flagMemProfile, "memprofile", "", "write memory profile to `file`")
	flags.IntVar(&snc.flags.flagDeadlock, "debug-deadlock", 0, "enable deadlock detection with given timeout")
}

func (snc *Agent) checkFlags(osArgs []string) {
	err := snc.flagset.Parse(osArgs)
	if err != nil {
		LogStderrf("ERROR: %s", err.Error())
	}

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
			LogStderrf("ERROR: either use -debug-profile or -cpu/memprofile, not both")
			os.Exit(ExitCodeError)
		}

		runtime.SetBlockProfileRate(BlockProfileRateInterval)
		runtime.SetMutexProfileFraction(BlockProfileRateInterval)

		go func() {
			// make sure we log panics properly
			defer snc.logPanicExit()

			server := &http.Server{
				Addr:              snc.flags.flagProfile,
				ReadTimeout:       DefaultSocketTimeout * time.Second,
				ReadHeaderTimeout: DefaultSocketTimeout * time.Second,
				WriteTimeout:      DefaultSocketTimeout * time.Second,
				IdleTimeout:       DefaultSocketTimeout * time.Second,
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
			LogStderrf("ERROR: could not create CPU profile: %s", err.Error())
			os.Exit(ExitCodeError)
		}

		if err := pprof.StartCPUProfile(cpuProfileHandler); err != nil {
			LogStderrf("ERROR: could not start CPU profile: %s", err.Error())
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

func (snc *Agent) initListener(conConf *ConfigSection, handler RequestHandler) (*Listener, error) {
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
		log.Errorf("failed to start %s listener: %s", name, err.Error())
		listener.Stop()
		delete(snc.Listeners, name)

		return
	}

	log.Tracef("listener %s started", name)
}

// RunCheck calls check by name and returns the check result
func (snc *Agent) RunCheck(name string, args []string) *CheckResult {
	res := snc.runCheck(name, args)
	res.replaceOutputVariables()

	return res
}

func (snc *Agent) runCheck(name string, args []string) *CheckResult {
	log.Tracef("command: %s", name)
	log.Tracef("args: %#v", args)
	check, ok := AvailableChecks[name]
	if !ok {
		res := CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - No such check: %s", name),
		}

		return &res
	}

	res, err := check.Handler.Check(snc, args)
	if err != nil {
		res := CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - %s", err.Error()),
		}

		return &res
	}

	return res
}

func (snc *Agent) buildStartupMsg() string {
	platform, _, pversion, err := host.PlatformInformation()
	if err != nil {
		log.Debugf("failed to get platform information: %s", err.Error())
	}
	hostid, err := os.Hostname()
	if err != nil {
		log.Debugf("failed to get platform host id: %s", err.Error())
	}
	msg := fmt.Sprintf("%s starting (version:v%s.%s - build:%s - host:%s - pid:%d - os:%s %s - arch:%s)",
		NAME, VERSION, snc.Revision, snc.Build, hostid, os.Getpid(), platform, pversion, runtime.GOARCH)

	return msg
}

func (snc *Agent) createLogger(config *Config) {
	conf := snc.Config.Section("/settings/log")
	if config != nil {
		conf = config.Section("/settings/log")
	}

	snc.applyLogLevel(conf)
	setLogFile(snc, conf)
}

func (snc *Agent) applyLogLevel(conf *ConfigSection) {
	level, ok := conf.GetString("level")
	if !ok {
		level = "info"
	}

	switch {
	case snc.flags.flagVeryVerbose, snc.flags.flagTraceVerbose:
		level = "trace"
	case snc.flags.flagVerbose:
		level = "debug"
	}
	setLogLevel(level)
}
