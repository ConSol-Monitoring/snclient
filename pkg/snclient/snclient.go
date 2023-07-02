package snclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
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
	VERSION = "0.03"

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

var (
	// Build contains the current git commit id
	// compile passing -ldflags "-X pkg/snclient.Build <build sha1>" to set the id.
	Build = ""

	// Revision contains the minor version number (number of commits, since last tag)
	// compile passing -ldflags "-X pkg/snclient.Revision <commits>" to set the revision number.
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

var (
	AvailableTasks     []*LoadableModule
	AvailableListeners []*LoadableModule

	GlobalMacros = getGlobalMacros()

	// macros can be either ${...} or %(...)
	reMacro = regexp.MustCompile(`\$\{\s*[a-zA-Z\-_: ]+\s*\}|%\(\s*[a-zA-Z\-_: ]+\s*\)`)

	// runtime macros can be %...%
	reRuntimeMacro = regexp.MustCompile(`(?:%|\$)[a-zA-Z\-_: ]+(?:%|\$)`)
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
	Config            *Config     // reference to global config object
	Listeners         *ModuleSet  // Listeners stores if we started listeners
	Tasks             *ModuleSet  // Tasks stores if we started task runners
	Counter           *CounterSet // Counter stores collected counters from tasks
	flags             *AgentFlags
	cpuProfileHandler *os.File
	initSet           *AgentRunSet
	osSignalChannel   chan os.Signal
	running           atomic.Bool
}

// AgentRunSet contains the initial startup config items
type AgentRunSet struct {
	config    *Config
	listeners *ModuleSet
	tasks     *ModuleSet
	files     []string
}

// NewAgent returns a new Agent object ready to be started by Run()
func NewAgent(flags *AgentFlags) *Agent {
	snc := &Agent{
		Listeners: NewModuleSet("listener"),
		Tasks:     NewModuleSet("task"),
		Counter:   NewCounterSet(),
		Config:    NewConfig(),
		flags:     flags,
	}
	snc.checkFlags()
	snc.createLogger(nil)

	// reads the args, check if they are params, if so sends them to the configuration reader
	initSet, err := snc.init()
	if err != nil {
		LogStderrf("ERROR: %s", err.Error())
		snc.CleanExit(ExitCodeError)
	}
	snc.initSet = initSet
	snc.Tasks = initSet.tasks
	snc.Config = initSet.config
	snc.createLogger(initSet.config)

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
	snc.Listeners.StopRemove()
}

func (snc *Agent) startModules(initSet *AgentRunSet) {
	if snc.Tasks != initSet.tasks {
		snc.Tasks.StopRemove()
	}
	if snc.Listeners != initSet.listeners {
		snc.Listeners.StopRemove()
	}

	snc.Config = initSet.config
	snc.Listeners = initSet.listeners
	snc.Tasks = initSet.tasks

	snc.Tasks.Start()
	snc.Listeners.Start()

	snc.initSet = initSet
}

func (snc *Agent) init() (*AgentRunSet, error) {
	var files configFiles
	files = snc.flags.ConfigFiles

	defaultLocations := []string{
		"./snclient.ini",
		"/etc/snclient/snclient.ini",
		path.Join(GlobalMacros["exe-path"], "snclient.ini"),
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

func (snc *Agent) readConfiguration(files []string) (*AgentRunSet, error) {
	config := NewConfig()
	for _, path := range files {
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
		pathSection.Set("exe-path", GlobalMacros["exe-path"])
	}

	// set defaults for empty path settings
	for _, key := range []string{"exe-path", "shared-path", "scripts", "certificate-path"} {
		val, ok := pathSection.GetString(key)
		if !ok || val == "" {
			pathSection.Set(key, pathSection.data["exe-path"])
		}
	}

	// replace macros in path section early
	for key, val := range pathSection.data {
		val = ReplaceMacros(val, pathSection.data, GlobalMacros)
		pathSection.Set(key, val)
	}

	// replace other sections
	for _, section := range config.sections {
		for key, val := range section.data {
			val = ReplaceMacros(val, pathSection.data, GlobalMacros)
			section.Set(key, val)
		}
	}

	for key, val := range pathSection.data {
		log.Tracef("conf macro: %s -> %s", key, val)
	}

	tasks, err := snc.initModules("tasks", AvailableTasks, config)
	if err != nil {
		return nil, fmt.Errorf("task initialization failed: %s", err.Error())
	}

	listen, err := snc.initModules("listener", AvailableListeners, config)
	if err != nil {
		return nil, fmt.Errorf("listener initialization failed: %s", err.Error())
	}

	if len(listen.modules) == 0 {
		log.Warnf("no listener enabled")
	}

	return &AgentRunSet{
		config:    config,
		listeners: listen,
		tasks:     tasks,
		files:     files,
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
			log.Errorf("%s: %s", entry.ConfigKey, err.Error())

			continue
		}

		name := entry.Name()
		if listener, ok := mod.(RequestHandler); ok {
			name = listener.BindString()
		}

		err = modules.Add(name, mod)
		if err != nil {
			log.Errorf("%s: %s", entry.ConfigKey, err.Error())

			continue
		}
	}

	return modules, nil
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
	return fmt.Sprintf("v%s.%s", VERSION, Revision)
}

// PrintVersion prints the version.
func (snc *Agent) PrintVersion() {
	fmt.Fprintf(os.Stdout, "%s %s (Build: %s)\n", NAME, snc.Version(), Build)
}

func (snc *Agent) checkFlags() {
	if snc.flags.ProfilePort != "" {
		if snc.flags.ProfileCPU != "" || snc.flags.ProfileMem != "" {
			LogStderrf("ERROR: either use -debug-profile or -cpu/memprofile, not both")
			os.Exit(ExitCodeError)
		}

		runtime.SetBlockProfileRate(BlockProfileRateInterval)
		runtime.SetMutexProfileFraction(BlockProfileRateInterval)

		go func() {
			// make sure we log panics properly
			defer snc.logPanicExit()

			server := &http.Server{
				Addr:              snc.flags.ProfilePort,
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

// RunCheck calls check by name and returns the check result
func (snc *Agent) RunCheck(name string, args []string) *CheckResult {
	res := snc.runCheck(name, args)
	res.Finalize()

	return res
}

func (snc *Agent) runCheck(name string, args []string) *CheckResult {
	log.Tracef("command: %s", name)
	log.Tracef("args: %#v", args)
	check, ok := AvailableChecks[name]
	if !ok {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - No such check: %s", name),
		}
	}

	res, err := check.Handler.Check(snc, args)
	if err != nil {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("${status} - %s", err.Error()),
		}
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
		NAME, VERSION, Revision, Build, hostid, os.Getpid(), platform, pversion, runtime.GOARCH)

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

	if snc.flags.Quiet {
		level = "error"
	}

	if snc.flags.LogLevel != "" {
		level = snc.flags.LogLevel
	}

	switch {
	case snc.flags.Verbose >= 2:
		level = "trace"
	case snc.flags.Verbose >= 1:
		level = "debug"
	}
	setLogLevel(level)
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
	log.Tracef("running as %s, moving updated file to %s", executable, binPath)

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
		output, err := cmd.CombinedOutput()
		log.Tracef("[update] net stop snclient: %s", strings.TrimSpace(string(output)))
		if err != nil {
			log.Debugf("net stop snclient failed: %s", err.Error())
		}
	}

	// move the file in place
	err = os.Rename(tmpPath, binPath)
	if err != nil {
		log.Errorf("move: %s", err.Error())

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
	files = append(files, snc.initSet.files...)
	files = append(files, binFile)
	ticker := time.NewTicker(500 * time.Millisecond)
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

/* replaceMacros replaces variables in given string.
 * possible macros are:
 *   ${macro}
 *   %(macro)
 */
func ReplaceMacros(value string, macroSets ...map[string]string) string {
	value = reMacro.ReplaceAllStringFunc(value, func(str string) string {
		orig := str
		str = strings.TrimSpace(str)

		switch {
		// ${...} macros
		case strings.HasPrefix(str, "${"):
			str = strings.TrimPrefix(str, "${")
			str = strings.TrimSuffix(str, "}")
		// %(...) macros
		case strings.HasPrefix(str, "%("):
			str = strings.TrimPrefix(str, "%(")
			str = strings.TrimSuffix(str, ")")
		}
		str = strings.TrimSpace(str)

		return getMacrosetsValue(str, orig, macroSets...)
	})

	return value
}

/* ReplaceRuntimeMacros replaces runtime variables in given string.
 * possible macros are:
 *   %macro%
 *   $macro$
 */
func ReplaceRuntimeMacros(value string, macroSets ...map[string]string) string {
	value = reRuntimeMacro.ReplaceAllStringFunc(value, func(str string) string {
		orig := str
		str = strings.TrimSpace(str)

		switch {
		// %...% macros
		case strings.HasPrefix(str, "%"):
			str = strings.TrimPrefix(str, "%")
			str = strings.TrimSuffix(str, "%")
		// $...$ macros
		case strings.HasPrefix(str, "$"):
			str = strings.TrimPrefix(str, "$")
			str = strings.TrimSuffix(str, "$")
		}

		return getMacrosetsValue(str, orig, macroSets...)
	})

	return value
}

func getMacrosetsValue(macro, orig string, macroSets ...map[string]string) string {
	flag := ""
	flags := strings.SplitN(macro, ":", 1)
	if len(flags) == 2 {
		macro = flags[0]
		flag = strings.ToLower(flags[1])
	}

	for _, ms := range macroSets {
		if repl, ok := ms[macro]; ok {
			switch flag {
			case "lc":
				return strings.ToLower(repl)
			case "uc":
				return strings.ToUpper(repl)
			default:
				return repl
			}
		}
	}

	return orig
}

func setProcessErrorResult(err error) (output string) {
	if os.IsNotExist(err) {
		output = "UNKNOWN: Return code of 127 is out of bounds. Make sure the plugin you're trying to run actually exists."

		return
	}
	if os.IsPermission(err) {
		output = "UNKNOWN: Return code of 126 is out of bounds. Make sure the plugin you're trying to run is executable."

		return
	}
	log.Errorf("system error: %w", err)
	output = fmt.Sprintf("UNKNOWN: %s", err.Error())

	return
}

func fixReturnCodes(output *string, exitCode *int64, state *os.ProcessState) {
	if *exitCode >= 0 && *exitCode <= 3 {
		return
	}
	if *exitCode == 126 {
		*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds. Make sure the plugin you're trying to run is executable.\n%s", *exitCode, *output)
		*exitCode = 2

		return
	}
	if *exitCode == 127 {
		*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds. Make sure the plugin you're trying to run actually exists.\n%s", *exitCode, *output)
		*exitCode = 2

		return
	}
	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		if waitStatus.Signaled() {
			*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds. Plugin exited by signal: %s.\n%s", waitStatus.Signal(), waitStatus.Signal(), *output)
			*exitCode = 2

			return
		}
	}
	*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds.\n%s", *exitCode, *output)
	*exitCode = 3
}

func (snc *Agent) runExternalCommand(command string, timeout int64) (stdout, stderr string, exitCode int64, proc *os.ProcessState, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmdList := []string{"/bin/sh", "-c", command}
	if runtime.GOOS == "windows" {
		cmdList = utils.Tokenize(command)
		cmdList, err = utils.TrimQuotesAll(cmdList)
		if err != nil {
			return "", "", ExitCodeUnknown, nil, fmt.Errorf("proc: %s", err.Error())
		}
	}
	log.Tracef("exec.Command: %#v", cmdList)
	cmd := exec.CommandContext(ctx, cmdList[0], cmdList[1:]...) //nolint:gosec // tainted input is configurable

	// byte buffer for output
	var errbuf bytes.Buffer
	var outbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	// prevent child from receiving signals meant for the agent only
	setSysProcAttr(cmd)

	err = cmd.Start()
	if err != nil && cmd.ProcessState == nil {
		return "", "", ExitCodeUnknown, nil, fmt.Errorf("proc: %s", err.Error())
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
