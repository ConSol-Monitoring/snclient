package snclient

import (
	"bytes"
	"fmt"
	"io"
	standardlog "log"
	"os"
	"path/filepath"
	"runtime"
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
	restoreLevel      string
)

func setLogLevel(level string) {
	restoreLevel = level
	switch strings.ToLower(level) {
	case "off":
		log.SetMinMaxSeverity(factorlog.StringToSeverity("PANIC"), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityNone)
	case "error":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityDefault)
	case "info":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityDefault)
	case "debug":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityDebug)
	case "trace":
		log.SetMinMaxSeverity(factorlog.StringToSeverity(strings.ToUpper(level)), factorlog.StringToSeverity("PANIC"))
		log.SetVerbosity(LogVerbosityTrace)
	case "":
	default:
		log.Errorf("unknown log level: %s", level)
	}
}

func raiseLogLevel(level string) {
	if factorlog.StringToSeverity(strings.ToUpper(level)) < factorlog.StringToSeverity(strings.ToUpper(restoreLevel)) {
		prev := restoreLevel
		setLogLevel(level)
		restoreLevel = prev
	}
}

func disableLogsTemporarily() {
	prev := restoreLevel
	setLogLevel("off")
	restoreLevel = prev
}

func restoreLogLevel() {
	setLogLevel(restoreLevel)
}

func setLogFile(snc *Agent, conf *ConfigSection) {
	file, _ := conf.GetString("file name")
	// override from cmd flags
	if snc.flags.LogFile != "" {
		file = snc.flags.LogFile
	}

	logColorOn := LogColors
	logColorReset := LogColorReset
	if runtime.GOOS == "windows" {
		logColorOn = ""
		logColorReset = ""
	}

	var logFormatter factorlog.Formatter
	switch file {
	case "stdout", "":
		logFormatter = BuildFormatter(logColorOn + DateTimeLogFormat + LogFormat + logColorReset)
		targetWriter = os.Stdout
	case "stderr":
		logFormatter = BuildFormatter(logColorOn + DateTimeLogFormat + LogFormat + logColorReset)
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

	if IsInteractive() {
		if targetWriter != os.Stdout && targetWriter != os.Stderr {
			doOnce.Do(func() {
				abs, _ := filepath.Abs(file)
				fmt.Fprintf(os.Stdout, snc.buildStartupMsg()+"\n")
				fmt.Fprintf(os.Stdout, "further logs will go into: %s\n", abs)
			})
		}
	}

	format, _ := conf.GetString("format")
	switch {
	case format != "":
		logFormatter = BuildFormatter(format)
	case snc.flags.LogFormat != "":
		logFormatter = BuildFormatter(snc.flags.LogFormat)
	}

	if runtime.GOOS == "windows" {
		targetWriter = NewWindowsLineEndingWriter(targetWriter)
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

func LogError2(_ interface{}, err error) {
	if err != nil {
		logErr := log.Output(factorlog.ERROR, 2, err.Error())
		if logErr != nil {
			LogStderrf("failed to log: %s (%s)", err.Error(), logErr.Error())
		}
	}
}

func LogDebug(err error) {
	if err != nil {
		logErr := log.Output(factorlog.DEBUG, 2, err.Error())
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

func (l *LogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	callLevel := 2

	switch strings.ToLower(l.level) {
	case "error":
		err = log.Output(factorlog.ERROR, callLevel, msg)
	case "warn":
		err = log.Output(factorlog.WARN, callLevel, msg)
	case "info":
		err = log.Output(factorlog.INFO, callLevel, msg)
	}

	if err != nil {
		return 0, fmt.Errorf("log: %s", err.Error())
	}

	return len(msg), nil
}

func NewLogWriter(level string) *LogWriter {
	l := new(LogWriter)
	l.level = level

	return l
}

func NewStandardLog(level string) *standardlog.Logger {
	writer := NewLogWriter(level)
	logger := standardlog.New(writer, "", 0)

	return logger
}

// Custom writer that replaces \n with \r\n
type WindowsLineEndingWriter struct {
	writer io.Writer
}

func NewWindowsLineEndingWriter(writer io.Writer) *WindowsLineEndingWriter {
	return &WindowsLineEndingWriter{writer: writer}
}

func (w *WindowsLineEndingWriter) Write(p []byte) (int, error) {
	// Replace all occurrences of \n with \r\n in the input
	p = bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))

	return w.writer.Write(p) //nolint:wrapcheck // just a simple wrapper
}
