package snclient

import (
	"fmt"
	"io/fs"
	"syscall"
	"time"
)

func getCheckFileTimes(fileInfo fs.FileInfo) (*FileInfoUnified, error) {
	fileInfoSys, ok := fileInfo.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return nil, fmt.Errorf("type assertion for fileInfo.Sys() failed")
	}

	return &FileInfoUnified{
		Atime: time.Unix(0, fileInfoSys.LastAccessTime.Nanoseconds()),
		Mtime: time.Unix(0, fileInfoSys.LastWriteTime.Nanoseconds()),
		Ctime: time.Unix(0, fileInfoSys.CreationTime.Nanoseconds()),
	}, nil
}
