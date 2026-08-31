//go:build windows

package snclient

import (
	"fmt"
	"io/fs"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileStandardInfo is the output of GetFileInformationByHandleEx when called with the FileStandardInfo info class.
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_standard_info
type fileStandardInfo struct {
	AllocationSize int64
	EndOfFile      int64
	NumberOfLinks  uint32
	DeletePending  bool
	Directory      bool
}

// getFileDiskSize returns the actual disk size of the file or directory.
// The win32 file attributes used by os.FileInfo.Sys() do not include the allocated size, so an additional API call is required.
// The file is opened without FILE_FLAG_OPEN_REPARSE_POINT, so symlinks are resolved and the size of the target is returned.
func getFileDiskSize(_ fs.FileInfo, path string) (uint64, error) {
	// GetFileInformationByHandleEx requires the handle to be opened with the
	// FILE_READ_ATTRIBUTES access right, an invalid-handle error is returned otherwise.
	// 8.3 short names (ex.: C:\Users\RUNNER~1) are resolved to their long form first,
	// since short paths can also cause the query to fail.
	longPath, err := resolveLongPath(path)
	if err != nil {
		longPath = path
	}

	pathPtr, err := windows.UTF16PtrFromString(longPath)
	if err != nil {
		return 0, fmt.Errorf("could not convert path to UTF16: %s", longPath)
	}

	handle, err := windows.CreateFile(pathPtr, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, fmt.Errorf("could not open file %s: %s", longPath, err.Error())
	}
	defer LogDebug(windows.CloseHandle(handle))

	var info fileStandardInfo
	err = windows.GetFileInformationByHandleEx(handle, windows.FileStandardInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		return 0, fmt.Errorf("could not get file information for %s: %s", longPath, err.Error())
	}
	if info.AllocationSize < 0 {
		return 0, fmt.Errorf("invalid negative allocation size: %d", info.AllocationSize)
	}

	return uint64(info.AllocationSize), nil
}
