//go:build linux

package snclient

import (
	"syscall"
)

func getInode(fileName string) uint64 {
	var fileStats syscall.Stat_t
	err := syscall.Stat(fileName, &fileStats)
	if err != nil {
		return 0
	}

	return fileStats.Ino
}
