package snclient

import (
	"fmt"
	"io/fs"
	"syscall"
	"time"

	fileversion "github.com/bi-zone/go-fileversion"
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

func getFileVersion(path string) (string, error) {
	f, err := fileversion.New(path)
	if err != nil {
		return "0.0.0.0", fmt.Errorf("fileversion failed for %s: %s", path, err.Error())
	}

	return f.FixedInfo().FileVersion.String(), nil
}
