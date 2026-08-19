//go:build !windows

package snclient

// Credentials can only be stored in the Windows Credential Manager.
// On other platforms all credential functions are no-ops.

func addShareCredential(cred *Credential) error {
	log.Debugf("credentials: storing credentials is only supported on windows, skipping target %s", cred.Target)

	return nil
}

func addShareConnection(_ *Credential, shareRoot string) error {
	log.Debugf("credentials: network share connections are only supported on windows, skipping target %s", shareRoot)

	return nil
}

func deleteShareConnection(_ string) error {
	return nil
}

func currentUserDomain() string {
	return ""
}
