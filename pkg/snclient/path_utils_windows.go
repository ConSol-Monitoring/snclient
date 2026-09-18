//go:build windows

package snclient

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// resolveLongPath expands 8.3 short names (ex.: C:\Users\RUNNER~1) to their long form.
// Some Win32 APIs (e.g. GetFileInformationByHandleEx with the FileStandardInfo class)
// fail with an invalid-handle error when given a short path, so paths are resolved to
// their long form before use.
func resolveLongPath(path string) (string, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", fmt.Errorf("GetLongName error when creating UTF16 string pointer from path %w", err)
	}
	size, _ := windows.GetLongPathName(pathPtr, nil, 0)
	if size == 0 {
		return "", fmt.Errorf("GetLongPathName returned no size for %s", path)
	}
	buf := make([]uint16, size)
	res, _ := windows.GetLongPathName(pathPtr, &buf[0], size)
	if res == 0 {
		return "", fmt.Errorf("GetLongPathName returned 0 for %s", path)
	}

	return windows.UTF16ToString(buf[:res]), nil
}
