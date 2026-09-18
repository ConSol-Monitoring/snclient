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

func isLink(fi fs.FileInfo) bool {
	return fi.Mode()&fs.ModeSymlink != 0
}

func getFileInode(fi fs.FileInfo) (uint64, bool) {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}

	return stat.Ino, true
}

// POSIX st_blocks is always reported in 512 byte units, this is independent of the filesystem block size
const statBlockSizeBytes = 512

func getFileDiskSize(fileInfo fs.FileInfo, _ string) (uint64, error) {
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("type assertion for fileInfo.Sys() failed")
	}
	if stat.Blocks < 0 {
		return 0, fmt.Errorf("invalid negative block count: %d", stat.Blocks)
	}

	return uint64(stat.Blocks) * statBlockSizeBytes, nil
}
