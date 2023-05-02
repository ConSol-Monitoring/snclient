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

	// LogColors sets colors for some log levels
	LogColors = `%{Color "yellow+b" "WARN"}` +
		`%{Color "red+b" "ERROR"}` +
		`%{Color "red+b" "FATAL"}` +
		`%{Color "white+b" "INFO"}` +
		`%{Color "white" "DEBUG"}` +
		`%{Color "white" "TRACE"}`

	// LogColorReset resets colors from LogColors
	LogColorReset = `%{Color "reset"}`
)

var doOnce sync.Once

var (
	DateTimeLogFormat = `[%{Date} %{Time "15:04:05.000"}]`
	LogFormat         = `[%{Severity}][pid:%{Pid}][%{ShortFile}:%{Line}] %{Message}`
	log               = factorlog.New(os.Stdout, BuildFormatter(DateTimeLogFormat+LogFormat))
	targetWriter      io.Writer
)

func CreateLogger(snc *Agent, config *Config) {
	conf := snc.Config.Section("/settings/log")
	if config != nil {
		conf = config.Section("/settings/log")
	}

	setLogLevel(snc, conf)
	setLogFile(snc, conf)
}

func setLogLevel(snc *Agent, conf *ConfigSection) {
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
	file, _ := conf.GetString("file name")
	// override from cmd flags
	if snc.flags.flagLogFile != "" {
		file = snc.flags.flagLogFile
	}

	var logFormatter factorlog.Formatter
	switch file {
	case "stdout", "":
		logFormatter = BuildFormatter(LogColors + DateTimeLogFormat + LogFormat + LogColorReset)
		targetWriter = os.Stdout
	case "stderr":
		logFormatter = BuildFormatter(LogColors + DateTimeLogFormat + LogFormat + LogColorReset)
		targetWriter = os.Stderr
	case "stdout-journal":
		logFormatter = BuildFormatter(LogFormat)
		targetWriter = os.Stdout
	default:
		logFormatter = BuildFormatter(DateTimeLogFormat + LogFormat)
		fHandle, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			log.Errorf(fmt.Sprintf("failed to open logfile %s: %s", file, err.Error()))

			return
		}
		targetWriter = fHandle
	}

	if isInteractive() {
		if targetWriter != os.Stdout && targetWriter != os.Stderr {
			doOnce.Do(func() {
				abs, _ := filepath.Abs(file)
				fmt.Fprintf(os.Stdout, snc.BuildStartupMsg()+"\n")
				fmt.Fprintf(os.Stdout, "further logs will go into: %s\n", abs)
			})
		}
	}

	format, _ := conf.GetString("format")
	switch {
	case format != "":
		logFormatter = BuildFormatter(format)
	case snc.flags.flagLogFormat != "":
		logFormatter = BuildFormatter(snc.flags.flagLogFormat)
	}

	log.SetFormatter(logFormatter)
	log.SetOutput(targetWriter)
}

func BuildFormatter(format string) *factorlog.StdFormatter {
	format = strings.ReplaceAll(format, "%{Pid}", fmt.Sprintf("%d", os.Getpid()))

	return (factorlog.NewStdFormatter(format))
}

func LogError(err error) {
	if err != nil {
		logErr := log.Output(factorlog.ERROR, 2, err.Error())
		if logErr != nil {
			LogStderrf("failed to log: %s (%s)", err.Error(), logErr.Error())
		}
	}
}

func LogStderrf(format string, args ...interface{}) {
	log.SetOutput(os.Stderr)
	logErr := log.Output(factorlog.ERROR, 2, fmt.Sprintf(format, args...))
	if logErr != nil {
		LogStderrf("failed to log: %s", logErr.Error())
	}
	log.SetOutput(targetWriter)
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
