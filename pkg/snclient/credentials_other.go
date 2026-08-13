//go:build !windows

package snclient

// Credentials can only be stored in the Windows Credential Manager.
// On other platforms all credential functions are no-ops.

func addShareCredential(cred *Credential) error {
	log.Debugf("credentials: storing credentials is only supported on windows, skipping target %s", cred.Target)

	return nil
}

func deleteShareCredential(_ string) error {
	return nil
}

func hasShareCredential(_ string) bool {
	return false
}

func currentUserDomain() string {
	return ""
}
