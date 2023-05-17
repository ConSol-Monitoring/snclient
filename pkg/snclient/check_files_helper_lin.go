//go:build linux

package snclient

import (
	"fmt"
	"io/fs"
	"syscall"
	"time"
)

func getCheckFileTimes(fileInfo fs.FileInfo) (*FileInfoUnified, error) {
	fileInfoSys, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("type assertion for fileInfo.Sys() failed")
	}

	return &FileInfoUnified{
		Atime: time.Unix(fileInfoSys.Atim.Sec, fileInfoSys.Atim.Nsec),
		Mtime: time.Unix(fileInfoSys.Mtim.Sec, fileInfoSys.Mtim.Nsec),
		Ctime: time.Unix(fileInfoSys.Ctim.Sec, fileInfoSys.Ctim.Nsec),
	}, nil
}
