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
	DRIVE_UNKNOWN     GetDriveTypeReturnValuePrimitive = 0
	DRIVE_NO_ROOT_DIR GetDriveTypeReturnValuePrimitive = 1
	DRIVE_REMOVABLE   GetDriveTypeReturnValuePrimitive = 2
	DRIVE_FIXED       GetDriveTypeReturnValuePrimitive = 3
	DRIVE_REMOTE      GetDriveTypeReturnValuePrimitive = 4
	DRIVE_CDROM       GetDriveTypeReturnValuePrimitive = 5
	DRIVE_RAMDISK     GetDriveTypeReturnValuePrimitive = 6
)

// windows.getDriveType returns a value, use this to return a string representation
func (driveType GetDriveTypeReturnValuePrimitive) toString() string {
	switch driveType {
	case DRIVE_UNKNOWN:
		return "unknown"
	case DRIVE_NO_ROOT_DIR:
		return "no_root_dir"
	case DRIVE_REMOVABLE:
		return "removable"
	case DRIVE_FIXED:
		return "fixed"
	case DRIVE_REMOTE:
		return "remote"
	case DRIVE_CDROM:
		return "cdrom"
	case DRIVE_RAMDISK:
		return "ramdisk"
	}
	return "unknown"
}

type WNetGetConnectionWReturnValuePrimitive uint32

var (
	kernel32Dll = windows.NewLazySystemDLL("Kernel32.dll")

	// [in] nBufferLength The maximum size of the buffer pointed to by lpBuffer, in TCHARs. This value includes space for the terminating null character. If this parameter is zero, lpBuffer is not used.
	// [out] lpBuffer A pointer to a buffer that receives a series of null-terminated strings, one for each valid drive in the system, plus with an additional null character. Each string is a device name.
	getLogicalDriveStringsW = kernel32Dll.NewProc("GetLogicalDriveStringsW")

	// [in, optional] lpRootPathName The root directory for the drive. A trailing backslash is required. If this parameter is NULL, the function uses the root of the current directory.
	getDriveTypeW = kernel32Dll.NewProc("GetDriveTypeW")

	winnetwkDll = windows.NewLazySystemDLL("Mpr.dll")

	// [in] lpLocalName Pointer to a constant null-terminated string that specifies the name of the local device to get the network name for.
	// [out] lpRemoteName Pointer to a null-terminated string that receives the remote name used to make the connection.
	// [in, out] lpnLength Pointer to a variable that specifies the size of the buffer pointed to by the lpRemoteName parameter, in characters. If the function fails because the buffer is not large enough, this parameter returns the required buffer size.
	wNetGetConnectionW = winnetwkDll.NewProc("WNetGetConnectionW")
)

func GetLogicalDriveStrings(nBufferLength uint32) (logicalDrives []string, err error) {
	// We are using getLogicalDriveStringsW , which uses Long Width Chars

	// bufferLength is in TCHARs
	var nBufferLengthCopy = nBufferLength
	var lpBuffer = make([]uint16, nBufferLengthCopy)
	returnValue, _, err := getLogicalDriveStringsW.Call(
		uintptr(unsafe.Pointer(&nBufferLengthCopy)),
		uintptr(unsafe.Pointer(&lpBuffer[0])),
	)

	if returnValue == 0 {
		return []string{}, fmt.Errorf("GetLogicalDriveStringsW returned error: %w", err)
	}

	if returnValue > uintptr(nBufferLength) {
		log.Debugf("GetLogicalDriveStringsW was called with a smaller buffer size than required, calling it recursively with size %d", returnValue)
		return GetLogicalDriveStrings(uint32(returnValue))
	}

	logicalDrives = make([]string, 0)

	if returnValue < uintptr(nBufferLength) {
		// There are multiple drives in the returned string, seperated by a NULL character
		for index := 0; uintptr(index) < returnValue; {
			decodedDriveStr := windows.UTF16ToString(lpBuffer[index:])
			logicalDrives = append(logicalDrives, decodedDriveStr)
			// log.Debugf("%s", decodedDriveStr)
			index += len(decodedDriveStr) + 1
		}
	}

	return logicalDrives, nil
}

func GetDriveType(lpRootPathName string) (returnValue GetDriveTypeReturnValuePrimitive, err error) {
	lpRootPathNameW16 := windows.StringToUTF16(lpRootPathName)

	rv, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(&lpRootPathNameW16[0])))

	return GetDriveTypeReturnValuePrimitive(rv), nil
}

func NetGetConnection(lpLocalName string) (lpRemoteName string, err error) {
	var lpLocalNameW16 []uint16 = windows.StringToUTF16(lpLocalName)

	var lpnLength uint32 = 32768
	lpRemoteNameW16 := make([]uint16, lpnLength)
	returnValue, _, err := wNetGetConnectionW.Call(
		uintptr(unsafe.Pointer(&lpLocalNameW16[0])),
		uintptr(unsafe.Pointer(&lpRemoteNameW16[0])),
		uintptr(unsafe.Pointer(&lpnLength)),
	)

	if returnValue != windows.NO_ERROR {
		if errors.Is(err, windows.ERROR_BAD_DEVICE) {
			return "", fmt.Errorf("The string pointed to by the lpLocalName parameter is invalid : %s", lpLocalName)
		} else if errors.Is(err, windows.ERROR_NOT_CONNECTED) {
			return "", fmt.Errorf("The device specified by lpLocalName is not a redirected device. For more information, see the following Remarks section.")
		} else if errors.Is(err, windows.ERROR_MORE_DATA) {
			return "", fmt.Errorf("The buffer is too small. The lpnLength parameter points to a variable that contains the required buffer size. More entries are available with subsequent calls.")
		} else if errors.Is(err, windows.ERROR_CONNECTION_UNAVAIL) {
			return "", fmt.Errorf("The device is not currently connected, but it is a persistent connection. For more information, see the following Remarks section.")
		} else if errors.Is(err, windows.ERROR_NO_NETWORK) {
			return "", fmt.Errorf(" The network is unavailable.")
		} else if errors.Is(err, windows.ERROR_EXTENDED_ERROR) {
			return "", fmt.Errorf("A network-specific error occurred. To obtain a description of the error, call the WNetGetLastError function. WNetGetLastError returned: %s", handleWNetError(returnValue, winnetwkDll))
		} else if errors.Is(err, windows.ERROR_NO_NET_OR_BAD_PATH) {
			return "", fmt.Errorf("None of the providers recognize the local name as having a connection. However, the network is not available for at least one provider to whom the connection may belong.")
		} else {
			return "", fmt.Errorf("WNetGetConnectionW returned an unrecognized error with value: %d", returnValue)
		}
	}

	lpRemoteName = windows.UTF16ToString(lpRemoteNameW16)

	return lpRemoteName, nil
}

func handleWNetError(errorCode uintptr, winnetwkDll *windows.LazyDLL) (err error) {
	wNetGetLastErrorAFunc := winnetwkDll.NewProc("WNetGetLastErrorA ")
	lpErrorBuf := make([]byte, 1024)
	lpNameBuf := make([]byte, 256)
	ret, _, _ := wNetGetLastErrorAFunc.Call(
		uintptr(errorCode),
		uintptr(unsafe.Pointer(&lpErrorBuf)),
		1024,
		uintptr(unsafe.Pointer(&lpNameBuf)),
		256,
	)
	if ret != windows.NO_ERROR {
		return fmt.Errorf("Got en error while getting the extended network error")
	}
	if ret == uintptr(windows.ERROR_INVALID_ADDRESS) {
		return fmt.Errorf("Provided an invalid buffer while getting the extended network error")
	}
	return nil
}
