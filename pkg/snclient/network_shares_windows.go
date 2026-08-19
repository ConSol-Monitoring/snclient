//go:build windows

package snclient

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// https://learn.microsoft.com/en-us/windows/win32/api/winnetwk/ns-winnetwk-netresourcew
	resourceTypeDisk = 0x1
)

var (
	winnetwkDll = windows.NewLazySystemDLL("Mpr.dll")

	// [in] lpLocalName Pointer to a constant null-terminated string that specifies the name of the local device to get the network name for.
	// [out] lpRemoteName Pointer to a null-terminated string that receives the remote name used to make the connection.
	// [in, out] lpnLength Pointer to a variable that specifies the size of the buffer pointed to by the lpRemoteName parameter,
	// in characters. If the function fails because the buffer is not large enough, this parameter returns the required buffer size.
	wNetGetConnectionW = winnetwkDll.NewProc("WNetGetConnectionW")

	// https://learn.microsoft.com/en-us/windows/win32/api/winnetwk/nf-winnetwk-wnetaddconnection2w
	// A pointer to a NETRESOURCE structure that specifies details of the proposed connection, such as information about the network resource, the local device, and the network resource provider.
	// Setting lpLocalName to null, the connection is established without mounting to a local device letter
	// [in] LPNETRESOURCEW lpNetResource,
	// A pointer to a constant null-terminated string that specifies a password to be used in making the network connection.
	// If lpPassword is NULL, the function uses the current default password associated with the user specified by the lpUserName parameter.
	// If lpPassword points to an empty string, the function does not use a password.
	// [in] LPCWSTR        lpPassword,
	// A pointer to a constant null-terminated string that specifies a user name for making the connection.
	// If lpUserName is NULL, the function uses the default user name. (The user context for the process provides the default user name.)
	// [in] LPCWSTR        lpUserName,
	// A set of connection options. The possible values for the connection options are defined in the Winnetwk.h header file. The following values can currently be used.
	// [in] DWORD          dwFlags
	wNetAddConnection2W = winnetwkDll.NewProc("WNetAddConnection2W")

	// https://learn.microsoft.com/en-us/windows/win32/api/winnetwk/nf-winnetwk-wnetcancelconnection2w
	// [in] LPCWSTR lpName, Pointer to a constant null-terminated string that specifies the name of either the redirected local device or the remote network resource to disconnect from.
	// [in] DWORD   dwFlags, connection type, irrelevant to our purposes.
	// [in] BOOL    fForce , Specifies whether the disconnection should occur if there are open files or jobs on the connection.
	wNetCancelConnection2W = winnetwkDll.NewProc("WNetCancelConnection2W")
)

// https://learn.microsoft.com/en-us/windows/win32/api/winnetwk/ns-winnetwk-netresourcew
// netResourceW is the NETRESOURCEW structure from winnetwk.h.
// do not reorder, the fields must match the Windows layout exactly.
// this is used in the wNetAddConnection2W call.
type netResourceW struct {
	// dwScope indicates the scope of the enumeration. This can be one of the RESOURCE_CONNECTED, RESOURCE_GLOBALNET or RESOURCE_CONTEXT values.
	dwScope uint32
	// dwType indicates the type of resource. This can be one of the
	// RESOURCETYPE_DISK, RESOURCETYPE_PRINT or RESOURCETYPE_ANY values.
	dwType uint32
	// dwDisplayType indicates how a provider wants a UI to display the resource.
	dwDisplayType uint32
	// dwUsage is a bitmask describing resource enumeration flags.
	dwUsage uint32
	// lpLocalName holds the name of a redirected local device when dwScope is RESOURCE_CONNECTED; otherwise it is undefined.
	lpLocalName *uint16
	// lpRemoteName holds the remote network name of the resource.
	lpRemoteName *uint16
	// lpComment is any provider-supplied comment about the resource.
	lpComment *uint16
	// lpProvider is the name of the provider that owns the resource.
	lpProvider *uint16
}

// addShareConnection establishes an SMB connection to the given share root using the supplied credentials.
// uses wNetAddConnection2W syscall
func addShareConnection(cred *Credential, shareRoot string) error {
	if cred.Type != CredentialTypeWindowsShare {
		return fmt.Errorf("unsupported credential type %q", cred.Type)
	}
	if cred.Username == "" {
		return fmt.Errorf("missing username")
	}

	shareRoot = strings.TrimSpace(shareRoot)
	if shareRoot == "" {
		return fmt.Errorf("empty share root")
	}

	remoteUTF16, err := syscall.UTF16PtrFromString(shareRoot)
	if err != nil {
		return fmt.Errorf("share root to utf16: %s", err.Error())
	}
	userUTF16, err := syscall.UTF16PtrFromString(cred.Username)
	if err != nil {
		return fmt.Errorf("username to utf16: %s", err.Error())
	}
	var passwordUTF16 *uint16
	if cred.Password != "" {
		passwordUTF16, err = syscall.UTF16PtrFromString(cred.Password)
		if err != nil {
			return fmt.Errorf("password to utf16: %s", err.Error())
		}
	}

	resource := netResourceW{
		dwType:       resourceTypeDisk,
		lpRemoteName: remoteUTF16,
	}

	ret, _, _ := wNetAddConnection2W.Call(
		uintptr(unsafe.Pointer(&resource)),
		uintptr(unsafe.Pointer(passwordUTF16)),
		uintptr(unsafe.Pointer(userUTF16)),
		0,
	)
	if ret != windows.NO_ERROR {
		return wNetError(ret, fmt.Sprintf("WNetAddConnection2W failed for %s", shareRoot))
	}

	return nil
}

// deleteShareConnection removes the SMB connection to the given share root again.
// uses wNetCancelConnection2W syscall
func deleteShareConnection(shareRoot string) error {
	shareRoot = strings.TrimSpace(shareRoot)
	if shareRoot == "" {
		return fmt.Errorf("empty share root")
	}

	shareRootUTF16, err := syscall.UTF16PtrFromString(shareRoot)
	if err != nil {
		return fmt.Errorf("share root to utf16: %s", err.Error())
	}

	ret, _, _ := wNetCancelConnection2W.Call(
		uintptr(unsafe.Pointer(shareRootUTF16)),
		0,
		1, // fForce: close the connection even if files are still open
	)
	if ret != windows.NO_ERROR {
		return wNetError(ret, fmt.Sprintf("WNetCancelConnection2W failed for %s", shareRoot))
	}

	return nil
}

// currentUserDomain returns the domain of the current user, e.g. CORP for CORP\svc.
// Local accounts report the computer name as their domain.
func currentUserDomain() string {
	var size uint32 = 256
	for {
		buffer := make([]uint16, size)
		err := windows.GetUserNameEx(windows.NameSamCompatible, &buffer[0], &size)
		if err == nil {
			name := syscall.UTF16ToString(buffer)
			if index := strings.IndexRune(name, '\\'); index > 0 {
				return name[:index]
			}

			return ""
		}
		if errors.Is(err, windows.ERROR_MORE_DATA) {
			continue
		}
		log.Debugf("credentials: GetUserNameEx failed: %s, using USERDOMAIN env var", err.Error())

		return os.Getenv("USERDOMAIN")
	}
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
	wNetGetLastErrorAFunc := winnetwkDll.NewProc("WNetGetLastErrorA")
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

// wNetError converts a WNet* return code into a descriptive error.
func wNetError(code uintptr, context string) error {
	switch code {
	case uintptr(windows.ERROR_ACCESS_DENIED):
		return fmt.Errorf("%s: access denied", context)
	case uintptr(windows.ERROR_BAD_NET_NAME):
		return fmt.Errorf("%s: the share name is invalid or the share does not exist", context)
	case uintptr(windows.ERROR_LOGON_FAILURE):
		return fmt.Errorf("%s: logon failure, the username or password is incorrect", context)
	case uintptr(windows.ERROR_INVALID_PASSWORD):
		return fmt.Errorf("%s: invalid password", context)
	case uintptr(windows.ERROR_BAD_USERNAME):
		return fmt.Errorf("%s: invalid username", context)
	case uintptr(windows.ERROR_ALREADY_ASSIGNED):
		return fmt.Errorf("%s: the resource is already connected", context)
	case uintptr(windows.ERROR_SESSION_CREDENTIAL_CONFLICT):
		return fmt.Errorf("%s: %w", context, errSessionCredentialConflict)
	case uintptr(windows.ERROR_NO_NET_OR_BAD_PATH):
		return fmt.Errorf("%s: the network path was not found or is not available", context)
	case uintptr(windows.ERROR_NOT_CONNECTED):
		return fmt.Errorf("%s: the connection does not exist", context)
	case uintptr(windows.ERROR_CONNECTION_UNAVAIL):
		return fmt.Errorf("%s: the connection is not currently available", context)
	case uintptr(windows.ERROR_OPEN_FILES):
		return fmt.Errorf("%s: the connection could not be closed because files are open", context)
	case uintptr(windows.ERROR_EXTENDED_ERROR):
		return fmt.Errorf("%s: %s", context, wNetGetLastErrorText(code))
	default:
		return fmt.Errorf("%s: unrecognized network error %d", context, code)
	}
}

// wNetGetLastErrorText returns the extended error description for a WNet* return code.
func wNetGetLastErrorText(code uintptr) string {
	wNetGetLastErrorW := winnetwkDll.NewProc("WNetGetLastErrorW")
	const errorBufLength = uint32(1024)
	const nameBufLength = uint32(256)
	errorBuf := make([]uint16, errorBufLength)
	nameBuf := make([]uint16, nameBufLength)
	ret, _, _ := wNetGetLastErrorW.Call(
		code,
		uintptr(unsafe.Pointer(&errorBuf[0])),
		uintptr(errorBufLength),
		uintptr(unsafe.Pointer(&nameBuf[0])),
		uintptr(len(nameBuf)),
	)
	if ret != windows.NO_ERROR {
		return fmt.Sprintf("extended network error %d", code)
	}

	return windows.UTF16ToString(errorBuf)
}
