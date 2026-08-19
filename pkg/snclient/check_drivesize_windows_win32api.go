//go:build windows

package snclient

import (
	"fmt"
	"unsafe"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"golang.org/x/sys/windows"
)

// The values for the constants are taken from the header file

type GetDriveTypeReturnValuePrimitive uint32

const (
	DriveUnknown   GetDriveTypeReturnValuePrimitive = 0
	DriveNoRootDir GetDriveTypeReturnValuePrimitive = 1
	DriveRemovable GetDriveTypeReturnValuePrimitive = 2
	DriveFixed     GetDriveTypeReturnValuePrimitive = 3
	DriveRemote    GetDriveTypeReturnValuePrimitive = 4
	DriveCdrom     GetDriveTypeReturnValuePrimitive = 5
	DriveRamdisk   GetDriveTypeReturnValuePrimitive = 6
)

// windows.getDriveType returns a value, use this to return a string representation
func (driveType GetDriveTypeReturnValuePrimitive) toString() string {
	switch driveType {
	case DriveUnknown:
		return "unknown"
	case DriveNoRootDir:
		return "no_root_dir"
	case DriveRemovable:
		return "removable"
	case DriveFixed:
		return "fixed"
	case DriveRemote:
		return "remote"
	case DriveCdrom:
		return "cdrom"
	case DriveRamdisk:
		return "ramdisk"
	}

	return "unknown"
}

type WNetGetConnectionWReturnValuePrimitive uint32

var (
	kernel32Dll = windows.NewLazySystemDLL("Kernel32.dll")

	// [in, optional] lpRootPathName The root directory for the drive. A trailing backslash is required.
	// If this parameter is NULL, the function uses the root of the current directory.
	getDriveTypeW = kernel32Dll.NewProc("GetDriveTypeW")
)

func GetDriveType(lpRootPathName string) (returnValue GetDriveTypeReturnValuePrimitive, err error) {
	if lpRootPathName == "" {
		return DriveUnknown, fmt.Errorf("lpRootPathName cannot be empty")
	}

	lpRootPathNameW16 := windows.StringToUTF16(lpRootPathName)

	ret, _, err := getDriveTypeW.Call(uintptr(unsafe.Pointer(&lpRootPathNameW16[0])))
	if err != nil && ret == 0 {
		log.Debugf("getDriveTypeW: Call returned an error: %s", err.Error())

		return DriveUnknown, nil
	}

	rvU, err := convert.UInt32E(ret)
	if err != nil {
		log.Debugf("getDriveTypeW: Call returned unexpected value: %s", err.Error())

		return DriveUnknown, nil
	}

	return GetDriveTypeReturnValuePrimitive(rvU), nil
}
