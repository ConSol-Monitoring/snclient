package snclient

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // default muxer is not exposed by default
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/consol-monitoring/snclient/pkg/counter"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/consol-monitoring/snclient/pkg/wmi"
	"github.com/kdar/factorlog"
	deadlock "github.com/sasha-s/go-deadlock"
	daemon "github.com/sevlyar/go-daemon"
	"github.com/shirou/gopsutil/v4/host"
)

const (
	// NAME contains the snclient full official name.
	NAME = "SNClient+"

	DESCRIPTION = "SNClient+ (Secure Naemon Client) is a general purpose" +
		" monitoring agent designed as replacement for NRPE and NSClient++."

	// VERSION contains the actual snclient version.
	VERSION = "0.31"

	// ExitCodeOK is used for normal exits.
	ExitCodeOK = 0

	// ExitCodeError is used for erroneous exits.
	ExitCodeError = 2

	// ExitCodeUnknown is used for unknown exits.
	ExitCodeUnknown = 3

	// BlockProfileRateInterval sets the profiling interval when started with -profile.
	BlockProfileRateInterval = 10

	// DefaultSocketTimeout sets the default timeout for tcp sockets.
	DefaultSocketTimeout = 60

	// DefaultCmdTimeout sets the default timeout for running commands.
	DefaultCmdTimeout = 30

	// DefaultProfilerTimeout sets the default timeout for pprof handler.
	DefaultProfilerTimeout = 180
)

var (
	// Build contains the current git commit id
	// compile passing -ldflags "-X snclient.Build <build sha1>" to set the id.
	Build = ""

	// Revision contains the minor version number (number of commits, since last tag)
	// compile passing -ldflags "-X snclient.Revision <commits>" to set the revision number.
	Revision = ""
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

// MainRunMode gives hints about wether snclient runs as server or not.
type MainRunMode int

const (
	_ MainRunMode = iota

	// ModeServer is used for long running modes
	ModeServer

	// ModeOneShot is used for single run commands
	ModeOneShot
)

var (
	AvailableTasks     []*LoadableModule
	AvailableListeners []*LoadableModule

	GlobalMacros = getGlobalMacros()
)

// https://github.com/golang/go/issues/8005#issuecomment-190753527
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type AgentFlags struct {
	ConfigFiles     []string
	Verbose         int
	Quiet           bool
	Help            bool
	Version         bool
	Mode            MainRunMode
	LogLevel        string
	LogFormat       string
	LogFile         string
	Pidfile         string
	ProfilePort     string
	ProfileCPU      string
	ProfileMem      string
	DeadlockTimeout int
}

type Agent struct {
	config            *Config      // reference to global config object
	Listeners         *ModuleSet   // Listeners stores if we started listeners
	Tasks             *ModuleSet   // Tasks stores if we started task runners
	Counter           *counter.Set // Counter stores collected counters from tasks
	flags             *AgentFlags
	cpuProfileHandler *os.File
	runSet            *AgentRunSet
	osSignalChannel   chan os.Signal
	running           atomic.Bool
	Log               *factorlog.FactorLog
	profileServer     *http.Server
}

// AgentRunSet contains the runtime dynamic references
type AgentRunSet struct {
	config     *Config
	listeners  *ModuleSet
	tasks      *ModuleSet
	files      []string
	cmdAliases map[string]CheckEntry // contains all registered check handler aliases
	cmdWraps   map[string]CheckEntry // contains all registered wrapped check handler
}

// NewAgent returns a new Agent object ready to be started by Run()
func NewAgent(flags *AgentFlags) *Agent {
	snc := &Agent{
		Listeners: NewModuleSet("listener"),
		Tasks:     NewModuleSet("task"),
		Counter:   counter.NewCounterSet(),
		config:    NewConfig(true),
		flags:     flags,
		Log:       log,
	}
	snc.checkFlags()
	snc.createLogger(nil)

	// reads the args, check if they are params, if so sends them to the configuration reader
	initSet, err := snc.Init()
	if initSet != nil {
		// create logger early, so start up errors can be added to the default log as well
		snc.createLogger(initSet.config)
		snc.checkConfigProfiler(initSet.config)
	}
	if err != nil {
		LogStderrf("ERROR: %s", err.Error())
		snc.CleanExit(ExitCodeError)
	}
	snc.runSet = initSet
	snc.Tasks = initSet.tasks
	snc.config = initSet.config

	snc.osSignalChannel = make(chan os.Signal, 1)

	log.Tracef("os args: %#v", os.Args)

	return snc
}

// IsRunning returns true if the agent is running
func (snc *Agent) IsRunning() bool {
	return snc.running.Load()
}

// Run starts the main loop and blocks until Stop() is called
func (snc *Agent) Run() {
	defer snc.logPanicExit()

	if snc.IsRunning() {
		log.Panicf("agent is already running")
	}

	log.Infof("%s", snc.buildStartupMsg())

	snc.createPidFile()
	defer snc.deletePidFile()

	signal.Notify(snc.osSignalChannel, syscall.SIGHUP)
	signal.Notify(snc.osSignalChannel, syscall.SIGTERM)
	signal.Notify(snc.osSignalChannel, os.Interrupt)
	signal.Notify(snc.osSignalChannel, syscall.SIGINT)
	setupUsrSignalChannel(snc.osSignalChannel)

	doOnce.Do(func() {
		if err := wmi.InitWbem(); err != nil {
			LogStderrf("ERROR: wmi initialization failed: %s", err.Error())
			snc.CleanExit(ExitCodeError)
		}
	})

	snc.startModules(snc.runSet)
	snc.running.Store(true)

	for {
		exitState := snc.mainLoop()
		if exitState != Reload {
			// make it possible to call mainLoop() from tests without exiting the tests
			break
		}
	}

	snc.running.Store(false)
	log.Infof("snclient exited (pid:%d)\n", os.Getpid())
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
	defer ticker.Stop()

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
				updateSet, err := snc.Init()
				if err != nil {
					log.Errorf("reloading configuration failed: %s", err.Error())

					continue
				}

				snc.createLogger(updateSet.config)
				snc.startModules(updateSet)

				return exitCode
			case Shutdown, ShutdownGraceFully:
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
	snc.Listeners.StopRemove()
}

func (snc *Agent) startModules(initSet *AgentRunSet) {
	if snc.Tasks != initSet.tasks {
		snc.Tasks.StopRemove()
	}
	if snc.Listeners != initSet.listeners {
		snc.Listeners.StopRemove()
	}

	snc.config = initSet.config
	snc.Listeners = initSet.listeners
	snc.Tasks = initSet.tasks

	snc.Tasks.Start()
	snc.Listeners.Start()

	snc.runSet = initSet
}

func (snc *Agent) Init() (*AgentRunSet, error) {
	var files configFiles
	files = snc.flags.ConfigFiles

	defaultLocations := []string{
		"./snclient.ini",
		"/etc/snclient/snclient.ini",
		filepath.Join(GlobalMacros["exe-path"], "snclient.ini"),
	}

	// no config supplied, check default locations, first match wins
	if len(files) == 0 {
		for _, file := range defaultLocations {
			_, err := os.Stat(file)
			if os.IsNotExist(err) {
				continue
			}
			// check if its readable
			fp, err := os.Open(file)
			if err != nil {
				continue
			}
			fp.Close()
			files = append(files, file)

			break
		}
	}

	// still empty
	if len(files) == 0 && snc.flags.Mode == ModeServer {
		return nil, fmt.Errorf("no config file supplied (--config=..) and no readable config file found in default locations (%s)",
			strings.Join(defaultLocations, ", "))
	}

	initSet, err := snc.readConfiguration(files)
	if err != nil {
		return initSet, err
	}

	initSet.tasks = NewModuleSet("tasks")
	err = snc.initModules("tasks", AvailableTasks, initSet, initSet.tasks)
	if err != nil {
		return initSet, fmt.Errorf("task initialization failed: %s", err.Error())
	}

	initSet.listeners = NewModuleSet("listener")
	if snc.flags.Mode == ModeServer {
		err = snc.initModules("listener", AvailableListeners, initSet, initSet.listeners)
		if err != nil {
			return initSet, fmt.Errorf("listener initialization failed: %s", err.Error())
		}

		if len(initSet.listeners.modules) == 0 {
			log.Warnf("no listener enabled")
		}
	}

	setScriptsRoot(initSet.config)

	return initSet, nil
}

func getGlobalMacros() map[string]string {
	execDir, execFile, execPath, err := utils.GetExecutablePath()
	if err != nil {
		LogStderrf("ERROR: could not detect path to executable: %s", err.Error())
		os.Exit(ExitCodeError)
	}

	// initialize global macros
	macros := map[string]string{
		"goos":     runtime.GOOS,
		"goarch":   runtime.GOARCH,
		"pkgarch":  pkgArch(runtime.GOARCH),
		"exe-path": execDir,
		"exe-file": execFile,
		"exe-full": execPath,
	}

	macros["file-ext"] = ""
	if runtime.GOOS == "windows" {
		macros["file-ext"] = ".exe"
	}

	return macros
}

func (snc *Agent) readConfiguration(files []string) (initSet *AgentRunSet, err error) {
	config := NewConfig(true)
	initSet = &AgentRunSet{
		config:     config,
		files:      files,
		cmdAliases: make(map[string]CheckEntry),
		cmdWraps:   make(map[string]CheckEntry),
	}
	var parseError error
	for _, path := range files {
		parseError = config.ReadINI(path, snc)
		if parseError != nil {
			break
		}
	}

	// apply defaults
	for sectionName, defaults := range DefaultConfig {
		section := config.Section(sectionName)
		section.MergeData(defaults)
	}

	// set defaults in path section
	pathSection := snc.setDefaultPaths(config, files)

	// build default macros
	config.DefaultMacros()

	// replace macros in path section early
	for key, val := range pathSection.data {
		val = ReplaceMacros(val, pathSection.data, GlobalMacros)
		pathSection.Set(key, val)
		(*config.defaultMacros)[key] = val
	}

	// shared path must exist
	if err = utils.IsFolder(pathSection.data["shared-path"]); err != nil {
		return initSet, fmt.Errorf("shared-path %s", err.Error())
	}

	// replace other sections
	for _, section := range config.sections {
		config.ReplaceMacrosDefault(section)
	}

	for key, val := range pathSection.data {
		log.Tracef("conf macro: %s -> %s", key, val)
	}

	if parseError != nil {
		return initSet, fmt.Errorf("reading settings failed: %s", parseError.Error())
	}

	return initSet, nil
}

func (snc *Agent) initModules(name string, loadable []*LoadableModule, runSet *AgentRunSet, modules *ModuleSet) error {
	conf := runSet.config

	modulesConf := conf.Section("/modules")
	for _, entry := range loadable {
		enabled, ok, err := modulesConf.GetBool(entry.ModuleKey)
		switch {
		case err != nil:
			return fmt.Errorf("error in %s /modules configuration: %s", entry.Name(), err.Error())
		case !ok:
			log.Tracef("%s %s is disabled by default config. skipping...", name, entry.Name())

			continue
		case !enabled:
			log.Tracef("%s %s is disabled by config. skipping...", name, entry.Name())

			continue
		}

		log.Tracef("init: %s %s", name, entry.Name())
		mod, err := entry.Init(snc, conf, runSet)
		if err != nil {
			return fmt.Errorf("%s: %s", entry.ConfigKey, err.Error())
		}

		name := entry.Name()
		if listener, ok := mod.(RequestHandler); ok {
			name = listener.BindString()
			log.Debugf("bind: %s", name)
		}

		err = modules.Add(name, mod)
		if err != nil {
			return fmt.Errorf("%s: %s", entry.ConfigKey, err.Error())
		}
	}

	return nil
}

func (snc *Agent) createPidFile() {
	// write the pid id if file path is defined
	if snc.flags.Pidfile == "" {
		return
	}
	// check existing pid
	if snc.checkStalePidFile() {
		LogStderrf("WARNING: removing stale pidfile %s", snc.flags.Pidfile)
	}

	err := os.WriteFile(snc.flags.Pidfile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600)
	if err != nil {
		LogStderrf("ERROR: Could not write pidfile: %s", err.Error())
		snc.CleanExit(ExitCodeError)
	}
}

func (snc *Agent) checkStalePidFile() bool {
	pid, err := utils.ReadPid(snc.flags.Pidfile)
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
	if snc.flags.Pidfile != "" {
		os.Remove(snc.flags.Pidfile)
	}
}

// Version returns version including Revision number
func (snc *Agent) Version() string {
	if Revision != "" {
		return fmt.Sprintf("v%s.%s", VERSION, Revision)
	}

	return fmt.Sprintf("v%s", VERSION)
}

// PrintVersion prints the version.
func (snc *Agent) PrintVersion() {
	fmt.Fprintf(os.Stdout, "%s %s (Build: %s, %s)\n", NAME, snc.Version(), Build, runtime.Version())
}

func (snc *Agent) checkFlags() {
	if snc.flags.ProfilePort != "" {
		if snc.flags.ProfileCPU != "" || snc.flags.ProfileMem != "" {
			LogStderrf("ERROR: either use -debug-profile or -cpu/memprofile, not both")
			os.Exit(ExitCodeError)
		}

		snc.startPProfiler(snc.flags.ProfilePort)
	}

	if snc.flags.ProfileCPU != "" {
		runtime.SetBlockProfileRate(BlockProfileRateInterval)

		cpuProfileHandler, err := os.Create(snc.flags.ProfileCPU)
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

	if snc.flags.DeadlockTimeout <= 0 {
		deadlock.Opts.Disable = true
	} else {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Duration(snc.flags.DeadlockTimeout) * time.Second
		deadlock.Opts.LogBuf = NewLogWriter("Error")
	}
}

func (snc *Agent) CleanExit(exitCode int) {
	snc.deletePidFile()

	if snc.flags.ProfileCPU != "" {
		pprof.StopCPUProfile()
		snc.cpuProfileHandler.Close()
		log.Infof("cpu profile written to: %s", snc.flags.ProfileCPU)
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

func (snc *Agent) logPanicRecover() {
	if r := recover(); r != nil {
		log.Errorf("********* PANIC *********")
		log.Errorf("Panic: %s", r)
		log.Errorf("**** Stack:")
		log.Errorf("%s", debug.Stack())
		log.Errorf("*************************")
	}
}

// RunCheck calls check by name and returns the check result
func (snc *Agent) RunCheck(name string, args []string) *CheckResult {
	return snc.RunCheckWithContext(context.TODO(), name, args)
}

// RunCheckWithContext calls check by name and returns the check result
func (snc *Agent) RunCheckWithContext(ctx context.Context, name string, args []string) *CheckResult {
	res := snc.runCheck(ctx, name, args)
	if res.Raw == nil || res.Raw.showHelp == 0 {
		res.Finalize()
	}

	return res
}

func (snc *Agent) runCheck(ctx context.Context, name string, args []string) *CheckResult {
	log.Tracef("command: %s", name)
	log.Tracef("args: %#v", args)
	check, ok := snc.getCheck(name)
	if !ok {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - No such check: %s", name),
		}
	}

	handler := check.Handler()
	chk := handler.Build()
	parsedArgs, err := chk.ParseArgs(args)
	if err != nil {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - %s", err.Error()),
		}
	}

	if chk.showHelp > 0 {
		state := CheckExitUnknown
		if chk.showHelp == Markdown {
			state = CheckExitOK
		}

		var help string
		switch builtin := handler.(type) {
		case *CheckBuiltin:
			help = builtin.Help(ctx, snc, chk, chk.showHelp)
		default:
			help = chk.Help(chk.showHelp)
		}

		return &CheckResult{
			Raw:    chk,
			State:  state,
			Output: help,
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(chk.timeout+1)*time.Second)
	defer cancel()

	res, err := handler.Check(ctx, snc, chk, parsedArgs)
	if err != nil {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - %s", err.Error()),
		}
	}

	return res
}

func (snc *Agent) getCheck(name string) (_ *CheckEntry, ok bool) {
	if snc.runSet != nil {
		if chk, ok := snc.runSet.cmdAliases[name]; ok {
			return &chk, ok
		}
		if chk, ok := snc.runSet.cmdWraps[name]; ok {
			return &chk, ok
		}
	}

	if chk, ok := AvailableChecks[name]; ok {
		return &chk, ok
	}

	return nil, false
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
	msg := fmt.Sprintf("%s starting (version:v%s.%s - build:%s - host:%s - pid:%d - os:%s %s - arch:%s - %s)",
		NAME, VERSION, Revision, Build, hostid, os.Getpid(), platform, pversion, runtime.GOARCH, runtime.Version())

	return msg
}

func (snc *Agent) createLogger(config *Config) {
	conf := snc.config.Section("/settings/log")
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

	if snc.flags.Quiet {
		level = "error"
	}

	if snc.flags.LogLevel != "" {
		level = snc.flags.LogLevel
	}

	trace2 := false

	// env beats config file
	env := os.Getenv("SNCLIENT_VERBOSE")
	switch env {
	case "":
	case "1":
		level = "verbose"
	case "2":
		level = "trace"
	case "3":
		level = "trace"
		trace2 = true
	default:
		level = env
	}

	// command line options beats env
	switch {
	case snc.flags.Verbose >= 3:
		level = "trace"
		trace2 = true
	case snc.flags.Verbose >= 2:
		level = "trace"
	case snc.flags.Verbose >= 1:
		level = "debug"
	}
	setLogLevel(level)
	if trace2 {
		log.SetVerbosity(LogVerbosityTrace2)
	}
}

// CheckUpdateBinary checks if we run as snclient.update.exe and if so, move that file in place and restart
func (snc *Agent) CheckUpdateBinary(mode string) {
	executable := GlobalMacros["exe-full"]
	updateFile := snc.buildUpdateFile(executable)

	if !strings.Contains(executable, ".update") {
		// remove update files, we might have started into that right now
		os.Remove(updateFile)

		return
	}

	binPath := strings.TrimSuffix(executable, GlobalMacros["file-ext"])
	binPath = strings.TrimSuffix(binPath, ".update")
	binPath += GlobalMacros["file-ext"]
	log.Debugf("started as %s, moving updated file to %s", executable, binPath)

	// create a copy of our update file which will be moved later
	tmpPath := binPath + ".tmp"
	defer os.Remove(tmpPath)
	err := utils.CopyFile(executable, tmpPath)
	if err != nil {
		log.Errorf("copy: %s", err.Error())

		return
	}

	if runtime.GOOS == "windows" && mode == "winservice" {
		// stop service, so we can replace the binary
		cmd := exec.Command("net", "stop", "snclient")
		output, err2 := cmd.CombinedOutput()
		log.Tracef("[update] net stop snclient: %s", strings.TrimSpace(string(output)))
		if err2 != nil {
			log.Debugf("net stop snclient failed: %s", err2.Error())
		}
	}

	// move the file in place
	err = os.Rename(tmpPath, binPath)
	if err != nil {
		log.Errorf("move update failed: %s", err.Error())

		return
	}

	snc.stop()
	snc.finishUpdate(binPath, mode)
}

func (snc *Agent) buildUpdateFile(executable string) string {
	return strings.TrimSuffix(executable, GlobalMacros["file-ext"]) + ".update" + GlobalMacros["file-ext"]
}

func (snc *Agent) restartWatcherCb(restartCb func()) {
	binFile := GlobalMacros["exe-full"]
	lastStat := map[string]*fs.FileInfo{}
	files := []string{}
	files = append(files, snc.runSet.files...)
	files = slices.AppendSeq(files, maps.Keys(snc.config.alreadyIncluded))
	files = append(files, binFile)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C
		if !snc.IsRunning() {
			return
		}

		for _, file := range files {
			stat, err := os.Stat(file)
			if err != nil {
				continue
			}
			last, ok := lastStat[file]
			if !ok {
				lastStat[file] = &stat

				continue
			}

			if stat.ModTime().After((*last).ModTime()) {
				log.Infof("%s has changed on disk, restarting...", file)
				restartCb()
			}
			lastStat[file] = &stat
		}
	}
}

func fixReturnCodes(output, stderr *string, exitCode *int64, timeout int64, procState *os.ProcessState, err error) {
	log.Tracef("stdout: %s", *output)
	log.Tracef("stderr: %s", *stderr)
	log.Tracef("exitCode: %d", *exitCode)
	log.Tracef("timeout: %d", timeout)
	log.Tracef("error: %#v", err)
	log.Tracef("state: %#v", procState)
	if err != nil {
		*exitCode = CheckExitUnknown
		switch {
		case errors.Is(err, fs.ErrNotExist):
			log.Warnf("plugin not found: %s", err.Error())
			*exitCode = 127
		case errors.Is(err, fs.ErrPermission):
			log.Warnf("no permissions: %s", err.Error())
			*exitCode = 126
		case errors.Is(err, context.DeadlineExceeded):
			*output = fmt.Sprintf("UNKNOWN - script run into timeout after %ds\n%s%s", timeout, *output, *stderr)
		case procState == nil:
			log.Warnf("system error: %s", err.Error())
			*output = fmt.Sprintf("UNKNOWN - %s", err.Error())
		default:
			*output = fmt.Sprintf("UNKNOWN - script error %s\n%s%s", err.Error(), *output, *stderr)
		}
	}

	if *exitCode >= 0 && *exitCode <= 3 {
		return
	}
	if *exitCode == 126 {
		*output = fmt.Sprintf("UNKNOWN - Return code of %d is out of bounds. Make sure the plugin you're trying to run is executable.\n%s", *exitCode, *output)
		*exitCode = CheckExitUnknown

		return
	}
	if *exitCode == 127 {
		cwd, _ := os.Getwd()
		*output = fmt.Sprintf("UNKNOWN - Return code of %d is out of bounds. Make sure the plugin you're trying to run actually exists (current working directory: %s).\n%s", *exitCode, cwd, *output)
		*exitCode = CheckExitUnknown

		return
	}
	if waitStatus, ok := procState.Sys().(syscall.WaitStatus); ok {
		if waitStatus.Signaled() {
			*output = fmt.Sprintf("UNKNOWN - Return code of %d is out of bounds. Plugin exited by signal: %s.\n%s", waitStatus.Signal(), waitStatus.Signal(), *output)
			*exitCode = CheckExitUnknown

			return
		}
	}

	*output = fmt.Sprintf("UNKNOWN - Return code of %d is out of bounds.\n%s", *exitCode, *output)
	*exitCode = CheckExitUnknown
}

func catchOutputErrors(command, stderr *string, exitCode *int64) {
	// cmd.exe did not find script
	if *exitCode != 0 {
		return
	}

	cmd := strings.Fields(*command)
	if !strings.HasSuffix(cmd[0], "cmd.exe") && !strings.HasSuffix(cmd[0], "powershell.exe") {
		return
	}

	// catch powershell errors which would result in exit code 0 otherwise
	if strings.Contains(*stderr, "CategoryInfo") &&
		strings.Contains(*stderr, "FullyQualifiedErrorId") {
		*exitCode = ExitCodeUnknown
	}
}

// runs check command (makes sure exit code is from 0-3)
func (snc *Agent) runExternalCheckString(ctx context.Context, command string, timeout int64) (stdout, stderr string, exitCode int64, err error) {
	cmd, err := snc.MakeCmd(ctx, command)
	var procState *os.ProcessState
	if err == nil {
		stdout, stderr, exitCode, procState, err = snc.runExternalCommand(ctx, cmd, timeout)
	}
	fixReturnCodes(&stdout, &stderr, &exitCode, timeout, procState, err)

	return stdout, stderr, exitCode, err
}

// runs command and does not touch exit code and such
func (snc *Agent) execCommand(ctx context.Context, command string, timeout int64) (stdout, stderr string, exitCode int64, err error) {
	cmd, err := snc.MakeCmd(ctx, command)
	if err == nil {
		stdout, stderr, exitCode, _, err = snc.runExternalCommand(ctx, cmd, timeout)
	}

	return stdout, stderr, exitCode, err
}

func (snc *Agent) runExternalCommand(ctx context.Context, cmd *exec.Cmd, timeout int64) (stdout, stderr string, exitCode int64, proc *os.ProcessState, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// byte buffer for output
	var errbuf bytes.Buffer
	var outbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	workDir, _ := snc.config.Section("/paths").GetString("shared-path")
	if err = utils.IsFolder(workDir); err != nil {
		return "", "", ExitCodeUnknown, nil, fmt.Errorf("invalid shared-path %s: %s", workDir, err.Error())
	}
	cmd.Dir = workDir
	err = cmd.Start()
	if err != nil && cmd.ProcessState == nil {
		return "", "", ExitCodeUnknown, nil, fmt.Errorf("proc: %w", err)
	}

	// https://github.com/golang/go/issues/18874
	// timeout does not work for child processes and/or if file handles are still open
	go procTimeoutGuard(ctx, snc, cmd.Process)

	err = cmd.Wait()
	cancel()
	if err != nil && cmd.ProcessState == nil {
		return "", "", ExitCodeUnknown, nil, fmt.Errorf("proc: %w", err)
	}

	state := cmd.ProcessState
	ctxErr := ctx.Err()
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return "", "", ExitCodeUnknown, state, fmt.Errorf("timeout: %w", ctxErr)
	}

	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		exitCode = int64(waitStatus.ExitStatus())
	}

	// extract stdout and stderr
	stdout = string(bytes.TrimSpace((bytes.Trim(outbuf.Bytes(), "\x00"))))
	stderr = string(bytes.TrimSpace((bytes.Trim(errbuf.Bytes(), "\x00"))))

	catchOutputErrors(&cmd.Path, &stderr, &exitCode)

	log.Tracef("exit: %d", exitCode)
	log.Tracef("stdout: %s", stdout)
	log.Tracef("stderr: %s", stderr)

	return stdout, stderr, exitCode, state, nil
}

func procTimeoutGuard(ctx context.Context, snc *Agent, proc *os.Process) {
	defer snc.logPanicExit()
	<-ctx.Done() // wait till command runs into timeout or is finished (canceled)
	if proc == nil {
		return
	}
	cmdErr := ctx.Err()
	switch {
	case errors.Is(cmdErr, context.DeadlineExceeded):
		// timeout
		processTimeoutKill(proc)
	case errors.Is(cmdErr, context.Canceled):
		// normal exit
		return
	}
}

func (snc *Agent) verifyPassword(confPassword, userPassword string) bool {
	// password checks are disabled
	if confPassword == "" {
		return true
	}

	// no login with default password
	if confPassword == DefaultPassword {
		log.Warnf("configured password in ini file matches default password, deny all access -> 403")

		return false
	}

	// hashed password?
	fields := strings.SplitN(confPassword, ":", 2)
	if len(fields) == 2 {
		switch strings.ToLower(fields[0]) {
		case "sha256":
			confPassword = fields[1]
			var err error
			userPassword, err = utils.Sha256Sum(userPassword)
			if err != nil {
				log.Errorf("sha256 failed: %s", err.Error())

				return false
			}
		default:
			log.Errorf("unsupported hash algorithm: %s", fields[0])
		}
	}

	if confPassword == userPassword {
		return true
	}

	log.Warnf("password mismatch -> 403")

	return false
}

// setDefaultPaths sets and returns defaults from the /paths config section
func (snc *Agent) setDefaultPaths(config *Config, configFiles []string) *ConfigSection {
	// set default exe-path path
	pathSection := config.Section("/paths")
	exe, ok := pathSection.GetString("exe-path")
	switch {
	case ok && exe != "":
		log.Warnf("exe-path should not be set manually, it will be overwritten anyway")

		fallthrough
	default:
		pathSection.Set("exe-path", GlobalMacros["exe-path"])
	}

	// set default shared-path path to base dir of first config file or current directory as fallback.
	shared, ok := pathSection.GetString("shared-path")
	if !ok || shared == "" {
		switch {
		case len(configFiles) > 0:
			pathSection.Set("shared-path", filepath.Dir(configFiles[0]))
		default:
			path, _ := os.Getwd()
			pathSection.Set("shared-path", path)
		}
	}

	// scripts points to %{shared-path}/scripts unless set otherwise
	scripts, ok := pathSection.GetString("scripts")
	if !ok || scripts == "" {
		pathSection.Set("scripts", filepath.Join(pathSection.data["shared-path"], "scripts"))
	}

	// scripts points to %{shared-path}/scripts unless set otherwise
	certs, ok := pathSection.GetString("certificate-path")
	if !ok || certs == "" {
		pathSection.Set("certificate-path", pathSection.data["shared-path"])
	}

	return pathSection
}

// MakeCmd returns the Cmd struct to execute the named program with
// the given arguments.
//
// It first tries to find relative filenames in the PATH or in ${scripts}
// If the first try did not succeed then it assumes we have a path with spaces
// and appends argument after argument until a valid command path is found.
func (snc *Agent) MakeCmd(ctx context.Context, command string) (*exec.Cmd, error) {
	// wrap the platform specific command builder
	cmd, err := snc.makeCmd(ctx, command)
	switch {
	case err != nil:
		return nil, err
	case cmd.Args != nil:
		log.Tracef("command object:\n path: %s\n args: %v\n SysProcAttr: %v\n", cmd.Path, cmd.Args, cmd.SysProcAttr)
	default:
		log.Tracef("command object:\n path: %s\n args: (none)\n SysProcAttr: %v\n", cmd.Path, cmd.SysProcAttr)
	}

	return cmd, err
}

// redirect log output from 3rd party process to main log file
func (snc *Agent) passthroughLogs(name, prefix string, logFn func(f string, v ...interface{}), pipeFn func() (io.ReadCloser, error)) {
	pipe, err := pipeFn()
	if err != nil {
		log.Errorf("failed to connect to %s: %s", name, err.Error())

		return
	}
	read := bufio.NewReader(pipe)
	go func() {
		defer snc.logPanicExit()
		for {
			line, _, err := read.ReadLine()
			if err != nil {
				break
			}

			lineStr := string(line)
			if len(line) > 0 {
				logFn("%s%s", prefix, lineStr)
			}
		}
	}()
}

// returns inventory structure
func (snc *Agent) BuildInventory(ctx context.Context, modules []string) map[string]interface{} {
	scripts := make([]string, 0)
	inventory := make(map[string]interface{})

	keys := make([]string, 0)
	for k := range AvailableChecks {
		keys = append(keys, k)
	}
	for k := range snc.runSet.cmdAliases {
		keys = append(keys, k)
	}
	for k := range snc.runSet.cmdWraps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		check, _ := snc.getCheck(k)
		handler := check.Handler()
		meta := handler.Build()
		if !meta.isImplemented(runtime.GOOS) {
			log.Debugf("skipping inventory for unimplemented (%s) check: %s / %s", runtime.GOOS, k, check.Name)

			continue
		}
		switch meta.hasInventory {
		case NoInventory:
			// skipped
		case ListInventory:
			name := strings.TrimPrefix(check.Name, "check_")
			if len(modules) > 0 && (!slices.Contains(modules, name)) {
				continue
			}
			meta.output = "inventory_json"
			meta.filter = ConditionList{{isNone: true}}
			data, err := handler.Check(ctx, snc, meta, []Argument{})
			if err != nil && (data == nil || data.Raw == nil) {
				log.Tracef("inventory %s returned error: %s", check.Name, err.Error())

				continue
			}

			inventory[name] = data.Raw.listData
		case NoCallInventory:
			name := strings.TrimPrefix(check.Name, "check_")
			if len(modules) > 0 && !slices.Contains(modules, name) {
				continue
			}
			inventory[name] = []interface{}{}
		case ScriptsInventory:
			scripts = append(scripts, check.Name)
		}
	}

	if len(modules) == 0 || slices.Contains(modules, "scripts") {
		inventory["scripts"] = scripts
	}

	if len(modules) == 0 || slices.Contains(modules, "exporter") {
		inventory["exporter"] = snc.listExporter()
	}

	hostID, err := os.Hostname()
	if err != nil {
		log.Errorf("failed to get host id: %s", err.Error())
	}

	return (map[string]interface{}{
		"inventory": inventory,
		"localtime": time.Now().Unix(),
		"snclient": map[string]interface{}{
			"version":  snc.Version(),
			"build":    Build,
			"arch":     runtime.GOARCH,
			"os":       runtime.GOOS,
			"hostname": hostID,
		},
	})
}

func (snc *Agent) getInventory(ctx context.Context, checkName string) (listData []map[string]string, err error) {
	checkName = strings.TrimPrefix(checkName, "check_")
	rawInv := snc.BuildInventory(ctx, []string{checkName})
	inv, ok := rawInv["inventory"]
	if !ok {
		return nil, fmt.Errorf("check %s not found in inventory", checkName)
	}

	if inv, ok := inv.(map[string]interface{}); ok {
		if list, ok := inv[checkName]; ok {
			if data, ok := list.([]map[string]string); ok {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("could not build inventory for %s", checkName)
}

func (snc *Agent) listExporter() (listData []map[string]string) {
	listData = make([]map[string]string, 0)
	for _, l := range snc.Listeners.modules {
		if j, ok := l.(ExporterListenerExposed); ok {
			listData = append(listData, j.JSON()...)
		}
	}

	return listData
}

func setScriptsRoot(config *Config) {
	// add script root
	scriptsSection := config.Section("/settings/external scripts")
	scriptRoot, ok := scriptsSection.GetString("script root")
	if ok {
		pathSection := config.Section("/paths")
		pathSection.Set("script root", scriptRoot)

		// reset cached default macros
		config.ResetDefaultMacros()
	}
}

func pkgArch(arch string) string {
	switch arch {
	case "386":
		return "i386"
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return "unknown"
	}
}

// check configuration if profiler needs to be stopped or started
func (snc *Agent) checkConfigProfiler(config *Config) {
	// do not touch profile server if started from command args
	if snc.flags.ProfilePort != "" {
		return
	}

	if enabled, _, _ := config.Section("/modules").GetBool("PProfiler"); !enabled {
		snc.stopPProfiler()

		return
	}

	port, _ := config.Section("/settings/PProfiler/server").GetString("port")
	if port == "" {
		snc.stopPProfiler()

		return
	}

	snc.startPProfiler(port)
}

// start the global profiler
func (snc *Agent) startPProfiler(port string) {
	if snc.profileServer != nil {
		log.Warnf("pprof profiler already listening at http://%s/debug/pprof/ (not starting again)", snc.profileServer.Addr)

		return
	}

	runtime.SetBlockProfileRate(BlockProfileRateInterval)
	runtime.SetMutexProfileFraction(BlockProfileRateInterval)

	go func() {
		// make sure we log panics properly
		defer snc.logPanicExit()

		server := &http.Server{
			Addr:              port,
			ReadTimeout:       DefaultSocketTimeout * time.Second,
			ReadHeaderTimeout: DefaultSocketTimeout * time.Second,
			WriteTimeout:      DefaultProfilerTimeout * time.Second,
			IdleTimeout:       DefaultSocketTimeout * time.Second,
			Handler:           http.DefaultServeMux,
		}

		log.Warnf("pprof profiler listening at http://%s/debug/pprof/ (make sure to use a binary without -trimpath, ex. from make builddebug)", port)
		err := server.ListenAndServe()
		snc.profileServer = server
		if err != nil {
			snc.profileServer = nil
			log.Debugf("http.ListenAndServe finished with: %e", err)
		}
	}()
}

// stop the global profiler
func (snc *Agent) stopPProfiler() {
	if snc.profileServer == nil {
		return
	}
	snc.profileServer.Close()
	snc.profileServer = nil
}

// counterCreate creates a new counter and adds some logging
func (snc *Agent) counterCreate(category, key string, bufferLength, interval time.Duration) {
	log.Tracef("creating counter %s.%s (buffer: %s)", category, key, bufferLength.String())
	snc.Counter.Create(category, key, bufferLength, interval)
}
