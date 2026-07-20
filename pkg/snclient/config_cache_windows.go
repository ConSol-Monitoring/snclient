//go:build windows

package snclient

import (
	"fmt"
	"os"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"golang.org/x/sys/windows"
)

func validateHTTPIncludeCacheFileOwner(_ string, _ os.FileInfo) error {
	// further validation is not possible on windows
	return nil
}

func getCurrentWindowsUser() (domain, account, sid string, err error) {
	// Get current process token.
	token := windows.Token(0)
	err = windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get current process: %w", err)
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user token : %w", err)
	}

	account, domain, _, err = user.User.Sid.LookupAccount("")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to lookup account for current user: %w", err)
	}

	return domain, account, user.User.Sid.String(), nil
}

func getCurrentUserHASH() (hash string, err error) {
	procDomain, procAccount, _, err := getCurrentWindowsUser()
	if err != nil {
		return "", fmt.Errorf("failed to get current windows user: %w", err)
	}

	hash, err = utils.Sha256Sum(fmt.Sprintf("%s/%s", procDomain, procAccount))
	if err != nil {
		return "", fmt.Errorf("failed to build hash sum: %w", err)
	}

	return hash, nil
}
