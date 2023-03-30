package snclient

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kdar/factorlog"
)

// define all available log level.
const (
	// LogVerbosityNone disables logging.
	LogVerbosityNone = 0

	// LogVerbosityDefault sets the default log level.
	LogVerbosityDefault = 1

	// LogVerbosityDebug sets the debug log level.
	LogVerbosityDebug = 2

	// LogVerbosityTrace sets trace log level.
	LogVerbosityTrace = 3
)

var doOnce sync.Once

var log = factorlog.New(os.Stdout, factorlog.NewStdFormatter(
	`[%{Date} %{Time "15:04:05.000"}]`+
		`[%{Severity}]`+
		`[pid:`+fmt.Sprintf("%d", os.Getpid())+`]`+
		`[%{ShortFile}:%{Line}] %{Message}`))

func CreateLogger(snc *Agent) {
	conf := snc.Config.Section("/settings/log")

	setLogLevel(snc, conf)
	setLogFile(snc, conf)
}

func setLogLevel(snc *Agent, conf *ConfigSection) {
	level, ok, err := conf.GetString("level")
	switch {
	case err != nil:
		log.Errorf("failed to read log level from config: %s")

		return
	case !ok:
		level = "info"
	}

	switch {
	case snc.flags.flagVeryVerbose, snc.flags.flagTraceVerbose:
		level = "trace"
	case snc.flags.flagVerbose:
		level = "debug"
	}

	switch strings.ToLower(level) {
	case "off":
		log.SetMinMaxSeverity(factorlog.StringToSeverity("PANIC"), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityNone)
	case "info":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityDefault)
	case "debug":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityDebug)
	case "trace":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityTrace)
	}
}

func setLogFile(snc *Agent, conf *ConfigSection) {
	file, _, err := conf.GetString("file name")
	switch {
	case err != nil:
		log.Errorf("failed to log file name from config: %s")

		return
	case snc.flags.flagLogFile != "":
		file = snc.flags.flagLogFile
	}

	var targetWriter io.Writer
	switch file {
	case "stdout", "":
		targetWriter = os.Stdout
	case "stderr":
		targetWriter = os.Stderr
	default:
		targetWriter, err = os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	}
	if err != nil {
		log.Errorf(fmt.Sprintf("failed to initialize logger: %s", err.Error()))

		return
	}

	o, _ := os.Stdout.Stat()
	// check if attached to terminal.
	if (o.Mode() & os.ModeCharDevice) == os.ModeCharDevice {
		if targetWriter != os.Stdout && targetWriter != os.Stderr {
			doOnce.Do(func() {
				abs, _ := filepath.Abs(file)
				fmt.Fprintf(os.Stdout, "further logs will go into: %s\n", abs)
			})
		}
	}

	log.SetOutput(targetWriter)
}

func LogError(err error) {
	if err != nil {
		logErr := log.Output(factorlog.ERROR, 2, err.Error())
		if logErr != nil {
			fmt.Fprintf(os.Stderr, "failed to log: %s (%s)", err.Error(), logErr.Error())
		}
	}
}

// LogWriter implements the io.Writer interface and simply logs everything with given level.
type LogWriter struct {
	level string
}

func (l *LogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))

	switch strings.ToLower(l.level) {
	case "error":
		log.Error(msg)
	case "warn":
		log.Warn(msg)
	case "info":
		log.Info(msg)
	}

	return 0, nil
}

func NewLogWriter(level string) *LogWriter {
	l := new(LogWriter)
	l.level = level

	return l
}
