//go:build windows

package snclient

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"golang.org/x/sys/windows"
)

const (
	// https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw
	credTypeDomainPassword = 2

	// the credential persists for the life of the logon session, it does not survive a reboot
	credPersistSession = 1
)

var advapi32 = windows.NewLazySystemDLL("advapi32.dll")

var (
	credWriteW  = advapi32.NewProc("CredWriteW")
	credReadW   = advapi32.NewProc("CredReadW")
	credDeleteW = advapi32.NewProc("CredDeleteW")
	credFree    = advapi32.NewProc("CredFree")
)

// credentialW is the CREDENTIALW structure from wincred.h.
// do not reorder, the fields must match the Windows layout exactly.
// https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// addShareCredential stores the credential in the Credential Manager of the current user.
// domain password credential is automatically used by the SMB redirector (NTLM/Kerberos) when connecting to the given target server.
func addShareCredential(cred *Credential) error {
	if cred.Type != CredentialTypeWindowsShare {
		return fmt.Errorf("unsupported credential type %q", cred.Type)
	}

	target := normalizeCredentialTargetFromUNCPath(cred.Target)
	if target == "" {
		return fmt.Errorf("empty credential target")
	}

	targetUTF16, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("target to utf16: %s", err.Error())
	}
	userUTF16, err := syscall.UTF16PtrFromString(cred.Username)
	if err != nil {
		return fmt.Errorf("username to utf16: %s", err.Error())
	}
	passwordUTF16, err := syscall.UTF16FromString(cred.Password)
	if err != nil {
		return fmt.Errorf("password to utf16: %s", err.Error())
	}
	// the credential blob contains the plaintext unicode password, no trailing null character
	passwordBlobSize, err := convert.UInt32E((len(passwordUTF16) - 1) * 2)
	if err != nil {
		return fmt.Errorf("password length to large for credential blob: %s", err.Error())
	}

	credential := credentialW{
		Type:               credTypeDomainPassword,
		TargetName:         targetUTF16,
		UserName:           userUTF16,
		CredentialBlobSize: passwordBlobSize,
		CredentialBlob:     (*byte)(unsafe.Pointer(&passwordUTF16[0])),
		Persist:            credPersistSession,
	}

	ret, _, err := credWriteW.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if ret == 0 {
		return fmt.Errorf("credWriteW failed: %s", err.Error())
	}

	return nil
}

// deleteShareCredential removes the domain password credential for the given target
// from the Credential Manager.
func deleteShareCredential(target string) error {
	target = normalizeCredentialTargetFromUNCPath(target)
	if target == "" {
		return fmt.Errorf("empty credential target")
	}

	targetUTF16, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("target to utf16: %s", err.Error())
	}

	ret, _, err := credDeleteW.Call(uintptr(unsafe.Pointer(targetUTF16)), uintptr(credTypeDomainPassword), 0)
	if ret == 0 {
		return fmt.Errorf("credDeleteW failed: %s", err.Error())
	}

	return nil
}

// hasShareCredential returns true if a domain password credential for the given target already exists in the Credential Manager.
func hasShareCredential(target string) bool {
	target = normalizeCredentialTargetFromUNCPath(target)
	if target == "" {
		return false
	}

	targetUTF16, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		log.Debugf("credentials: target to utf16: %s", err.Error())

		return false
	}

	var credential *credentialW
	// the flag is always 0
	ret, _, _ := credReadW.Call(
		uintptr(unsafe.Pointer(targetUTF16)),
		uintptr(credTypeDomainPassword),
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	if ret == 0 {
		return false
	}
	// the returned credential needs to be freed by this special CredFree function
	_, _, err = credFree.Call(uintptr(unsafe.Pointer(credential)))
	if err != nil {
		log.Debugf("credentials: credFree failed: %s", err.Error())
	}

	return true
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
