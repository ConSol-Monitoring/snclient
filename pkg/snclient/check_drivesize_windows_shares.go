package snclient

import (
	"bytes"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The LPSTR type and its alias PSTR specify a pointer to an array of 8-bit characters, which MAY be terminated by a null character.
// A 32-bit pointer to a string of 8-bit characters, which MAY be null-terminated.
// The docs say 32 bits, but a pointer should be uintptr sized ?
// type LPSTR uint32
type LPSTR uintptr

func lpstrToString(ptr LPSTR) string {
	if ptr == 0 {
		return ""
	}
	// windows.UTF16PtrToString takes *uint16, so we cast the uintptr
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr)))
}

// The values for the constants are taken from the header file
// On my machine I found one inside the windows development kit: "C:\Program Files (x86)\Windows Kits\10\Include\10.0.17763.0\um\winnetwk.h"

type DwScopePrimitive uint32

const (
	RESOURCE_CONNECTED  DwScopePrimitive = 0x00000001 // Enumerate all currently connected resources. The function ignores the dwUsage parameter. For more information, see the following Remarks section.
	RESOURCE_CONTEXT    DwScopePrimitive = 0x00000005 // Enumerate only resources in the network context of the caller. Specify this value for a Network Neighborhood view. The function ignores the dwUsage parameter.
	RESOURCE_GLOBALNET  DwScopePrimitive = 0x00000002 // Enumerate all resources on the network.
	RESOURCE_REMEMBERED DwScopePrimitive = 0x00000003 // Enumerate all remembered (persistent) connections. The function ignores the dwUsage parameter.
)

type DwTypePrimitive uint32

const (
	RESOURCETYPE_ANY   DwTypePrimitive = 0x00000000 // All resources. This value cannot be combined with RESOURCETYPE_DISK or RESOURCETYPE_PRINT.
	RESOURCETYPE_DISK  DwTypePrimitive = 0x00000001 // All disk resources.
	RESOURCETYPE_PRINT DwTypePrimitive = 0x00000002 // All print resources.
)

type DwUsagePrimitive uint32

const (
	RESOURCEUSAGE_CONNECTABLE DwUsagePrimitive = 0x00000001                                                                     // You can connect to the resource by calling NPAddConnection. If dwType is RESOURCETYPE_DISK, then, after you have connected to the resource, you can use the file system APIs, such as FindFirstFile, and FindNextFile, to enumerate any files and directories the resource contains.
	RESOURCEUSAGE_CONTAINER   DwUsagePrimitive = 0x00000002                                                                     //The resource is a container for other resources that can be enumerated by means of the NPOpenEnum, NPEnumResource, and NPCloseEnum functions. The container may, however, be empty at the time the enumeration is made. In other words, the first call to NPEnumResource may return WN_NO_MORE_ENTRIES.
	RESOURCEUSAGE_ATTACHED    DwUsagePrimitive = 0x00000010                                                                     // Setting this value forces WNetOpenEnum to fail if the user is not authenticated. The function fails even if the network allows enumeration without authentication.
	RESOURCEUSAGE_ALL         DwUsagePrimitive = (RESOURCEUSAGE_CONNECTABLE | RESOURCEUSAGE_CONTAINER | RESOURCEUSAGE_ATTACHED) // Setting this value is equivalent to setting RESOURCEUSAGE_CONNECTABLE, RESOURCEUSAGE_CONTAINER, and RESOURCEUSAGE_ATTACHED.
)

type DwDisplayTypePrimitive uint32

const (
	RESOURCEDISPLAYTYPE_NETWORK   DwDisplayTypePrimitive = 0x00000006 // The resource is a network provider.
	RESOURCEDISPLAYTYPE_DOMAIN    DwDisplayTypePrimitive = 0x00000001 // The resource is a collection of servers.
	RESOURCEDISPLAYTYPE_SERVER    DwDisplayTypePrimitive = 0x00000002 // The resource is a server.
	RESOURCEDISPLAYTYPE_SHARE     DwDisplayTypePrimitive = 0x00000003 // The resource is a directory.
	RESOURCEDISPLAYTYPE_DIRECTORY DwDisplayTypePrimitive = 0x00000009 // The resource is a directory.
	RESOURCEDISPLAYTYPE_GENERIC   DwDisplayTypePrimitive = 0x00000000 //The resource type is unspecified. This value is used by network providers that do not specify resource types.
)

type Netresource struct {
	dwScope       DwScopePrimitive
	dwType        DwTypePrimitive
	dwDisplayType DwDisplayTypePrimitive
	dwUsage       DwUsagePrimitive
	lpLocalName   LPSTR
	lpRemoteName  LPSTR
	lpComment     LPSTR
	lpProvider    LPSTR
}

const sizeofNetresource = (int32)(unsafe.Sizeof(Netresource{}))

var (
	winnetwkDll = windows.NewLazySystemDLL("Mpr.dll")

	// [in] dwScope Scope of the enumeration. This parameter can be one of the following values.
	// [in] dwType Resource types to be enumerated. This parameter can be a combination of the following values.
	// [in] dwUsage Resource usage type to be enumerated. This parameter can be a combination of the following values.
	// [in] lpNetResource Pointer to a NETRESOURCE structure that specifies the container to enumerate.
	// [out] lphEnum Pointer to an enumeration handle that can be used in a subsequent call to WNetEnumResource.
	wNetOpenEnumW = winnetwkDll.NewProc("WNetOpenEnumW")

	// Enumerate using the handle
	// [in] hEnum Handle that identifies an enumeration instance. This handle must be returned by the WNetOpenEnum function.
	// [in, out] lpcCount Pointer to a variable specifying the number of entries requested. If the number requested is –1, the function returns as many entries as possible. If the function succeeds, on return the variable pointed to by this parameter contains the number of entries actually read.
	// [out] lpBuffer Pointer to the buffer that receives the enumeration results. The results are returned as an array of NETRESOURCE structures. Note that the buffer you allocate must be large enough to hold the structures, plus the strings to which their members point. For more information, see the following Remarks section. The buffer is valid until the next call using the handle specified by the hEnum parameter. The order of NETRESOURCE structures in the array is not predictable.
	// [in, out] lpBufferSize Pointer to a variable that specifies the size of the lpBuffer parameter, in bytes. If the buffer is too small to receive even one entry, this parameter receives the required size of the buffer.
	wNetEnumREsourceW = winnetwkDll.NewProc("WNetEnumResourceW")

	// [in] hEnum Handle that identifies an enumeration instance. This handle must be returned by the WNetOpenEnum function.
	wNetCloseEnum = winnetwkDll.NewProc("WNetCloseEnum")
)

// The first time this function is called, use nil for the root
// If its not nill, it will start from the container resource being pointed by the root
func enumerateNetworkResources(rootNetresource *Netresource) (err error) {

	var lphEnum windows.Handle

	ret, _, _ := wNetOpenEnumW.Call(
		uintptr(RESOURCE_GLOBALNET),
		uintptr(RESOURCETYPE_DISK), // Only want disks, not printers
		uintptr(RESOURCEUSAGE_ALL),
		// On the first call, use NULL pointer,
		// Recursive calls use the memory address associated by go directly, without unsafe.Pointer
		// this is the pointer inside lpResourceBuffer
		uintptr(unsafe.Pointer(rootNetresource)),
		uintptr(unsafe.Pointer(&lphEnum)),
	)

	if ret != windows.NO_ERROR {
		return handleNetOpenEnumAReturnCode(ret, winnetwkDll)
	}

	// This will be populated during enumeration
	var lpBufferCapacity int32 = 65536 / sizeofNetresource
	lpNetResourceBuffer := make([]byte, lpBufferCapacity*(int32)(unsafe.Sizeof(Netresource{})))
	//var lpNetResourceBuffer []Netresource = make([]Netresource, lpBufferCapacity)

	var netEnumResourceErrorCode uintptr = 0
	for netEnumResourceErrorCode != (uintptr)(windows.ERROR_NO_MORE_ITEMS) {
		var lpCount int32 = -1 // is always used as -1 to list all resources

		netEnumResourceErrorCode, _, _ = wNetEnumREsourceW.Call(
			uintptr(lphEnum),
			uintptr(unsafe.Pointer(&lpCount)),
			uintptr(unsafe.Pointer(&lpNetResourceBuffer[0])),
			uintptr(unsafe.Pointer(&lpBufferCapacity)),
		)

		if netEnumResourceErrorCode == (uintptr)(windows.ERROR_NO_MORE_ITEMS) {
			break
		} else if ret != windows.NO_ERROR {
			return handleNetEnumResourceAReturnCode(ret, winnetwkDll)
		} else {
			for index := range lpCount {
				// have to refer back the memory as the struct
				indexPtr := unsafe.Pointer(&lpNetResourceBuffer[uintptr(index)*uintptr(sizeofNetresource)])
				extractedNetresource := (*Netresource)(indexPtr)

				log.Debugf("Information about the Netresource with the index %d:\n%s", index, displayStruct(extractedNetresource))

				// Bitwise and, capabilities are written with flags
				if (extractedNetresource.dwUsage & RESOURCEUSAGE_CONTAINER) != 0 {
					log.Debugf("This netresource with the index %d, recursively calling to discover shares", index)
					recursionErr := enumerateNetworkResources(extractedNetresource)
					if recursionErr != nil {
						return fmt.Errorf("error when recursively calling container network resource: %w", recursionErr)
					}
				}
			}

		}
	}

	netCloseEnumReturnValue, _, _ := wNetCloseEnum.Call(
		uintptr(lphEnum),
	)
	if netCloseEnumReturnValue != windows.NO_ERROR {
		return handleNetCloseEnumReturnCode(netCloseEnumReturnValue, winnetwkDll)
	}

	return nil
}

func (l *CheckDrivesize) setShares(requiredDisks map[string]map[string]string) {
	err := enumerateNetworkResources(nil)
	if err != nil {
		fmt.Println("Error when enumerating network resources: %s", err.Error())
	}
}

func handleNetOpenEnumAReturnCode(returnCode uintptr, winnetwkDll *windows.LazyDLL) (err error) {
	switch returnCode {
	case uintptr(windows.ERROR_NOT_CONTAINER):
		return fmt.Errorf(" The lpNetResource parameter does not point to a container. ")
	case uintptr(windows.ERROR_INVALID_PARAMETER):
		return fmt.Errorf("Either the dwScope or the dwType parameter is invalid, or there is an invalid combination of parameters. ")
	case uintptr(windows.ERROR_NO_NETWORK):
		return fmt.Errorf(" The network is unavailable. ")
	case uintptr(windows.ERROR_INVALID_ADDRESS):
		return fmt.Errorf("The lpNetResource parameter does not point to a container. ")
	case uintptr(windows.ERROR_EXTENDED_ERROR):
		return handleWNetError(returnCode, winnetwkDll)
	default:
		return fmt.Errorf("Unknown error code returned by NetOpenEnumA function: %d", returnCode)
	}
}

func handleNetEnumResourceAReturnCode(returnCode uintptr, winnetwkDll *windows.LazyDLL) (err error) {
	switch returnCode {
	case uintptr(windows.NO_ERROR):
		return fmt.Errorf("this error code means there are no errors to be processed.")
	case uintptr(windows.ERROR_NO_MORE_ITEMS):
		return fmt.Errorf("there are no more entries. The buffer contents are undefined.")
	case uintptr(windows.ERROR_MORE_DATA):
		return fmt.Errorf("More entries are available with subsequent calls. For more information, see the following Remarks section.")
	case uintptr(windows.ERROR_INVALID_HANDLE):
		return fmt.Errorf("The handle specified by the hEnum parameter is not valid. ")
	case uintptr(windows.ERROR_NO_NETWORK):
		return fmt.Errorf("The network is unavailable. (This condition is tested before hEnum is tested for validity.) ")
	case uintptr(windows.ERROR_EXTENDED_ERROR):
		return handleWNetError(returnCode, winnetwkDll)
	default:
		return fmt.Errorf("Unknown error code returned by NetEnumResourceA function: %d", returnCode)
	}
}

func handleNetCloseEnumReturnCode(returnCode uintptr, winnetwkDll *windows.LazyDLL) (err error) {
	switch returnCode {
	case uintptr(windows.ERROR_NO_NETWORK):
		return fmt.Errorf("The network is unavailable. (This condition is tested before the handle specified in the hEnum parameter is tested for validity.) ")
	case uintptr(windows.ERROR_INVALID_HANDLE):
		return fmt.Errorf("The hEnum parameter does not specify a valid handle.")
	case uintptr(windows.ERROR_EXTENDED_ERROR):
		return handleWNetError(returnCode, winnetwkDll)
	default:
		return fmt.Errorf("Unknown error code returned by NetCloseEnum function: %d", returnCode)
	}
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

// Converted this function from https://learn.microsoft.com/en-us/windows/win32/wnet/enumerating-network-resources
func displayStruct(netresource *Netresource) string {
	var buffer bytes.Buffer

	buffer.WriteString("NETRESOURCE Scope: ")
	switch netresource.dwScope {
	case RESOURCE_CONNECTED:
		buffer.WriteString("connected\n")
	case RESOURCE_GLOBALNET:
		buffer.WriteString("all resources\n")
	case RESOURCE_REMEMBERED:
		buffer.WriteString("remembered\n")
	default:
		buffer.WriteString(fmt.Sprintf("unknown scope %d\n", netresource.dwScope))
	}

	buffer.WriteString("NETRESOURCE Type: ")
	switch netresource.dwType {
	case RESOURCETYPE_ANY:
		buffer.WriteString("any\n")
	case RESOURCETYPE_DISK:
		buffer.WriteString("disk\n")
	case RESOURCETYPE_PRINT:
		buffer.WriteString("print\n")
	default:
		buffer.WriteString(fmt.Sprintf("unknown type %d\n", netresource.dwType))
	}

	buffer.WriteString("NETRESOURCE DisplayType: ")
	switch netresource.dwDisplayType {
	case RESOURCEDISPLAYTYPE_GENERIC:
		buffer.WriteString("generic\n")
	case RESOURCEDISPLAYTYPE_DOMAIN:
		buffer.WriteString("domain\n")
	case RESOURCEDISPLAYTYPE_SERVER:
		buffer.WriteString("server\n")
	case RESOURCEDISPLAYTYPE_SHARE:
		buffer.WriteString("share\n")
	case RESOURCEDISPLAYTYPE_DIRECTORY:
		buffer.WriteString("directory\n")
	case RESOURCEDISPLAYTYPE_NETWORK:
		buffer.WriteString("network\n")
	default:
		buffer.WriteString(fmt.Sprintf("unknown display type %d\n", netresource.dwDisplayType))
	}

	buffer.WriteString(fmt.Sprintf("NETRESOURCE Usage: 0x%x = ", netresource.dwUsage))
	if netresource.dwUsage&RESOURCEUSAGE_CONNECTABLE != 0 {
		buffer.WriteString("connectable ")
	}
	if netresource.dwUsage&RESOURCEUSAGE_CONTAINER != 0 {
		buffer.WriteString("container ")
	}
	buffer.WriteString("\n")

	buffer.WriteString(fmt.Sprintf("Local Name: %s\n", lpstrToString(netresource.lpLocalName)))
	buffer.WriteString(fmt.Sprintf("Remote Name: %s\n", lpstrToString(netresource.lpRemoteName)))
	buffer.WriteString(fmt.Sprintf("Comment: %s\n", lpstrToString(netresource.lpComment)))
	buffer.WriteString(fmt.Sprintf("Provider: %s\n", lpstrToString(netresource.lpProvider)))

	return buffer.String()
}
