package snclient

import (
	"os"
	"runtime/debug"

	log "github.com/sirupsen/logrus"
)

func OFFlogPanicExit() {
	if r := recover(); r != nil {
		log.Errorf("********* PANIC *********")
		log.Errorf("Panic: %s", r)
		log.Errorf("**** Stack:")
		log.Errorf("%s", debug.Stack())
		log.Errorf("*************************")
		os.Exit(ExitCodeError)
	}
}
