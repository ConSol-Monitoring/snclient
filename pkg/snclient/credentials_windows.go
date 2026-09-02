//go:build windows

package snclient

import (
	"fmt"
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

var (
	advapi32 = windows.NewLazySystemDLL("advapi32.dll")

	credWriteW = advapi32.NewProc("CredWriteW")
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
// uses credWriteW syscall
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
	// the credential blob contains the plaintext unicode password, no trailing null character
	// if no password was configured, store an empty blob
	passwordBlobSize := uint32(0)
	var passwordBlob *byte
	if cred.PasswordSet {
		passwordUTF16, e := syscall.UTF16FromString(cred.Password)
		if e != nil {
			return fmt.Errorf("password to utf16: %s", e.Error())
		}
		passwordBlobSize, err = convert.UInt32E((len(passwordUTF16) - 1) * 2)
		if err != nil {
			return fmt.Errorf("password length to large for credential blob: %s", err.Error())
		}
		passwordBlob = (*byte)(unsafe.Pointer(&passwordUTF16[0]))
	}

	credential := credentialW{
		Type:               credTypeDomainPassword,
		TargetName:         targetUTF16,
		UserName:           userUTF16,
		CredentialBlobSize: passwordBlobSize,
		CredentialBlob:     passwordBlob,
		Persist:            credPersistSession,
	}

	ret, _, err := credWriteW.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if ret == 0 {
		return fmt.Errorf("credWriteW failed: %s", err.Error())
	}

	return nil
}
