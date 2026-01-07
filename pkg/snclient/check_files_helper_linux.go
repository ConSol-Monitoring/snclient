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
		Atime: time.Unix(int64(fileInfoSys.Atim.Sec), int64(fileInfoSys.Atim.Nsec)), //nolint:unconvert // variable is platform specific and int32 on 386 arch
		Mtime: time.Unix(int64(fileInfoSys.Mtim.Sec), int64(fileInfoSys.Mtim.Nsec)), //nolint:unconvert // same
		Ctime: time.Unix(int64(fileInfoSys.Ctim.Sec), int64(fileInfoSys.Ctim.Nsec)), //nolint:unconvert // same
	}, nil
}

func getFileVersion(path string) (string, error) {
	return "0.0.0.0", fmt.Errorf("file version not supported: %s", path)
}
