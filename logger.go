package snclient

import (
	"fmt"
	"os"
	"strings"

	"github.com/kdar/factorlog"
)

// define all available log level.
const (
	LogLevelError  = 0
	LogLevelInfo   = 1
	LogLevelDebug  = 2
	LogLevelTrace  = 3
	LogLevelTrace2 = 4
)

var log = factorlog.New(os.Stdout, factorlog.NewStdFormatter(
	`[%{Date} %{Time "15:04:05.000"}]`+
		`[%{Severity}]`+
		`[pid:`+fmt.Sprintf("%d", os.Getpid())+`]`+
		`[%{ShortFile}:%{Line}] %{Message}`))

func CreateLogger(_ *Agent) {
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
