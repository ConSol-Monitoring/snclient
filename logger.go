package snclient

import (
	"fmt"
	"os"
	"path"
	"runtime/debug"

	"github.com/kdar/factorlog"
)

// define the available log level
const (
	LogLevelInfo   = 0
	LogLevelDebug  = 1
	LogLevelTrace  = 2
	LogLevelTrace2 = 3
)

func createLogger(config *configurationStruct) {
	// logging format
	frmt := `%{Color \"yellow\" \"WARN\"}%{Color \"red\" \"ERROR\"}%{Color \"red\" \"FATAL\"}[%{Date} %{Time "15:04:05.000"}][%{Severity}][%{File}:%{Line}] %{Message}`

	// check in config file if file is specified
	verbosity := getSeverity(config.debug)

	switch {
	case config.debug >= LogLevelTrace2 || config.logfile == "stderr":
		logger.SetOutput(os.Stderr)
	case config.logfile != "" && (config.logmode == "automatic" || config.logmode == "file"):
		file, err := openFileOrCreate(config.logfile)
		if err != nil {
			logger.Errorf("could not create or open file %s: %w", config.logfile, err)
		}
		file.Close()
		logfile, err := os.OpenFile(config.logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			logger.Errorf("could not open/create logfile: %w", err)
		}
		logger.SetOutput(logfile)
	default:
		logger.SetOutput(os.Stdout)
	}

	logger.SetFormatter(factorlog.NewStdFormatter(frmt))
	logger.SetMinMaxSeverity(factorlog.StringToSeverity(verbosity), factorlog.StringToSeverity("PANIC"))
}

func getSeverity(input int) string {
	if input > LogLevelTrace2 {
		input = LogLevelTrace2
	}
	switch input {
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelTrace, LogLevelTrace2:
		return "TRACE"
	}
	return "ERROR"
}

func logPanicExit() {
	if r := recover(); r != nil {
		logger.Errorf("********* PANIC *********")
		logger.Errorf("Panic: %s", r)
		logger.Errorf("**** Stack:")
		logger.Errorf("%s", debug.Stack())
		logger.Errorf("*************************")
		cleanExit(1)
	}
}

func setLogLevel(lvl int) {
	verbosity := getSeverity(lvl)
	logger.SetMinMaxSeverity(factorlog.StringToSeverity(verbosity), factorlog.StringToSeverity("PANIC"))
}

func disableLogging() {
	logger.SetMinMaxSeverity(factorlog.StringToSeverity("PANIC"), factorlog.StringToSeverity("PANIC"))
}
func openFileOrCreate(path string) (os.File, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		createDirectoryAndFile(path)
		file, err := os.Open(path)
		if err != nil {
			logger.Errorf("could not open file %s: %w", path, err)
			return *file, fmt.Errorf("open file %s failed: %w", path, err)
		}
		return *file, nil
	}
	file, err := os.Open(path)
	if err != nil {
		logger.Errorf("could not open file %s: %w", path, err)
	}
	return *file, nil
}

func createDirectoryAndFile(pathe string) {
	directory, file := path.Split(pathe)
	if directory != "" {
		err := os.MkdirAll(directory, 0755)
		if err != nil {
			logger.Panic(err)
		}
		_, err = os.Create(directory + "/" + file)
		if err != nil {
			logger.Panic(err)
		}
	} else {
		_, err := os.Create(file)
		if err != nil {
			logger.Panic(err)
		}
	}
}
