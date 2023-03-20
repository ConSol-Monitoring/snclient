package snclient

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/kdar/factorlog"
)

const (
	// VERSION contains the actual lmd version
	VERSION = "0.0.1"
)

// LogFormat sets the log format
var LogFormat string

func init() {
	LogFormat = `[%{Date} %{Time "15:04:05.000"}][%{Severity}][pid:` + fmt.Sprintf("%d", os.Getpid()) + `][%{ShortFile}:%{Line}] %{Message}`
}

var logger = factorlog.New(os.Stdout, factorlog.NewStdFormatter(LogFormat))

var prometheusListener *net.Listener
var pidFile string

func Daemon(build string) {
}

func deletePidFile(f string) {
	if f != "" {
		os.Remove(f)
	}
}

func cleanExit(exitCode int) {
	deletePidFile(pidFile)
	os.Exit(exitCode)
}

func logThreaddump() {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	if n < len(buf) {
		buf = buf[:n]
	}
	logger.Errorf("threaddump:\n%s", buf)
}
