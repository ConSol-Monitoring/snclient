//go:build windows

package snclient

import (
	"errors"
	"fmt"
	"unsafe"

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

	winnetwkDll = windows.NewLazySystemDLL("Mpr.dll")

	// [in] lpLocalName Pointer to a constant null-terminated string that specifies the name of the local device to get the network name for.
	// [out] lpRemoteName Pointer to a null-terminated string that receives the remote name used to make the connection.
	// [in, out] lpnLength Pointer to a variable that specifies the size of the buffer pointed to by the lpRemoteName parameter,
	//  in characters. If the function fails because the buffer is not large enough, this parameter returns the required buffer size.
	wNetGetConnectionW = winnetwkDll.NewProc("WNetGetConnectionW")
)

func GetDriveType(lpRootPathName string) (returnValue GetDriveTypeReturnValuePrimitive, err error) {
	if lpRootPathName == "" {
		return 0, fmt.Errorf("lpRootPathName cannot be empty")
	}

	lpRootPathNameW16 := windows.StringToUTF16(lpRootPathName)

	rv, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(&lpRootPathNameW16[0])))

	return GetDriveTypeReturnValuePrimitive(rv), nil
}

func NetGetConnection(lpLocalName string) (lpRemoteName string, err error) {
	if lpLocalName == "" {
		return "", fmt.Errorf("lpLocalName cannot be empty")
	}

	lpLocalNameW16 := windows.StringToUTF16(lpLocalName)

	var lpnLength uint32 = 32768
	lpRemoteNameW16 := make([]uint16, lpnLength)
	returnValue, _, err := wNetGetConnectionW.Call(
		uintptr(unsafe.Pointer(&lpLocalNameW16[0])),
		uintptr(unsafe.Pointer(&lpRemoteNameW16[0])),
		uintptr(unsafe.Pointer(&lpnLength)),
	)

	switch {
	case returnValue == windows.NO_ERROR:
		// this is what we want
	case errors.Is(err, windows.ERROR_BAD_DEVICE):
		return "", fmt.Errorf("the string pointed to by the lpLocalName parameter is invalid : %s", lpLocalName)
	case errors.Is(err, windows.ERROR_NOT_CONNECTED):
		return "", fmt.Errorf("the device specified by lpLocalName is not a redirected device. For more information, see the following Remarks section")
	case errors.Is(err, windows.ERROR_MORE_DATA):
		return "", fmt.Errorf("the buffer is too small. The lpnLength parameter points to a variable that contains the required buffer size. More entries are available with subsequent calls")
	case errors.Is(err, windows.ERROR_CONNECTION_UNAVAIL):
		return "", fmt.Errorf("the device is not currently connected, but it is a persistent connection. For more information, see the following Remarks section")
	case errors.Is(err, windows.ERROR_NO_NETWORK):
		return "", fmt.Errorf("the network is unavailable")
	case errors.Is(err, windows.ERROR_EXTENDED_ERROR):
		return "", fmt.Errorf("a network-specific error occurred. To obtain a description of the error, call the WNetGetLastError function."+
			"WNetGetLastError returned: %w", handleWNetError(returnValue, winnetwkDll))
	case errors.Is(err, windows.ERROR_NO_NET_OR_BAD_PATH):
		return "", fmt.Errorf("none of the providers recognize the local name as having a connection. However, the network is not available for at least one provider to whom the connection may belong")
	default:
		return "", fmt.Errorf("mNetGetConnectionW returned an unrecognized error with value: %d", returnValue)
	}

	lpRemoteName = windows.UTF16ToString(lpRemoteNameW16)

	return lpRemoteName, nil
}

func handleWNetError(errorCode uintptr, winnetwkDll *windows.LazyDLL) (err error) {
	wNetGetLastErrorAFunc := winnetwkDll.NewProc("WNetGetLastErrorA ")
	const lpErrorBufLength = uint32(1024)
	lpErrorBuf := make([]byte, lpErrorBufLength)
	const lpNameBufLength = uint32(256)
	lpNameBuf := make([]byte, lpNameBufLength)
	ret, _, _ := wNetGetLastErrorAFunc.Call(
		errorCode,
		uintptr(unsafe.Pointer(&lpErrorBuf)),
		uintptr(lpErrorBufLength),
		uintptr(unsafe.Pointer(&lpNameBuf)),
		uintptr(lpNameBufLength),
	)
	if ret != windows.NO_ERROR {
		return fmt.Errorf("got en error while getting the extended network error")
	}
	if ret == uintptr(windows.ERROR_INVALID_ADDRESS) {
		return fmt.Errorf("provided an invalid buffer while getting the extended network error")
	}

	return nil
}
