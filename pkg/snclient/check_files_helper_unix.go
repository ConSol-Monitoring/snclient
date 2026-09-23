//go:build freebsd || darwin || linux

package snclient

import (
	"fmt"
	"io/fs"
	"syscall"
)

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
