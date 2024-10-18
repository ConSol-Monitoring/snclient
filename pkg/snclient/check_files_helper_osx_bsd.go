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
		Atime: time.Unix(int64(fileInfoSys.Atimespec.Sec), int64(fileInfoSys.Atimespec.Nsec)), //nolint:unconvert // its a int32 on freebsd i386, so conversion is required
		Mtime: time.Unix(int64(fileInfoSys.Mtimespec.Sec), int64(fileInfoSys.Mtimespec.Nsec)), //nolint:unconvert // same
		Ctime: time.Unix(int64(fileInfoSys.Ctimespec.Sec), int64(fileInfoSys.Ctimespec.Nsec)), //nolint:unconvert // same
	}, nil
}

func getFileVersion(path string) (string, error) {
	return "0.0.0.0", fmt.Errorf("file version not supported: %s", path)
}
