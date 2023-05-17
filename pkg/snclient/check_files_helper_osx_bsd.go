//go:build freebsd || darwin

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
		Atime: time.Unix(int64(fileInfoSys.Atimespec.Sec), int64(fileInfoSys.Atimespec.Nsec)),
		Mtime: time.Unix(int64(fileInfoSys.Mtimespec.Sec), int64(fileInfoSys.Mtimespec.Nsec)),
		Ctime: time.Unix(int64(fileInfoSys.Ctimespec.Sec), int64(fileInfoSys.Ctimespec.Nsec)),
	}, nil
}
