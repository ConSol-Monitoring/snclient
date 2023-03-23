package snclient

import (
	"fmt"
	"os"

	"github.com/kdar/factorlog"
)

// define the available log level
const (
	LogLevelInfo   = 0
	LogLevelDebug  = 1
	LogLevelTrace  = 2
	LogLevelTrace2 = 3
)

var log = factorlog.New(os.Stdout, factorlog.NewStdFormatter(
	`[%{Date} %{Time "15:04:05.000"}]`+
		`[%{Severity}]`+
		`[pid:`+fmt.Sprintf("%d", os.Getpid())+`]`+
		`[%{ShortFile}:%{Line}] %{Message}`))

func CreateLogger(snc *Agent) {
	/*
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
			logfile, err := os.OpenFile(config.logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
			if err != nil {
				logger.Errorf("could not open/create logfile: %w", err)
			}
			logger.SetOutput(logfile)
		default:
			logger.SetOutput(os.Stdout)
		}

		logger.SetFormatter(factorlog.NewStdFormatter(frmt))
		logger.SetMinMaxSeverity(factorlog.StringToSeverity(verbosity), factorlog.StringToSeverity("PANIC"))
	*/
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

func setLogLevel(lvl int) {
	verbosity := getSeverity(lvl)
	log.SetMinMaxSeverity(factorlog.StringToSeverity(verbosity), factorlog.StringToSeverity("PANIC"))
}

func disableLogging() {
	log.SetMinMaxSeverity(factorlog.StringToSeverity("PANIC"), factorlog.StringToSeverity("PANIC"))
}
