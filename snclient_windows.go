package snclient

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

var elog debug.Log

type winService struct {
	snc       *Agent
	conf      *Config
	listeners map[string]*Listener
	tasks     *TaskSet
}

func (m *winService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	go m.snc.run(m.conf, m.listeners, m.tasks)
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				m.snc.stop()
				break loop
			default:
				elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func (snc *Agent) daemonize(config *Config, listeners map[string]*Listener, tasks *TaskSet) {
	const svcName = "snclient"
	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %s", err.Error())
	}
	if !inService {
		log.Fatalf("--daemon mode cannot run interactively")
	}

	elog, err = eventlog.Open(svcName)
	if err != nil {
		return
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("starting %s service", svcName))
	run := svc.Run
	err = run(svcName, &winService{
		snc:       snc,
		conf:      config,
		listeners: listeners,
		tasks:     tasks,
	})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", svcName, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", svcName))
}

func isInteractive() bool {
	inService, _ := svc.IsWindowsService()
	if inService {
		return false
	}
	return true
}

func setupUsrSignalChannel(osSignalUsrChannel chan os.Signal) {
}

func mainSignalHandler(sig os.Signal, snc *Agent) MainStateType {
	switch sig {
	case syscall.SIGTERM:
		log.Infof("got sigterm, quiting gracefully")

		return ShutdownGraceFully
	case os.Interrupt, syscall.SIGINT:
		log.Infof("got sigint, quitting")

		return Shutdown
	case syscall.SIGHUP:
		log.Infof("got sighup, reloading configuration...")

		return Reload
	default:
		log.Warnf("Signal not handled: %v", sig)
	}

	return Resume
}
