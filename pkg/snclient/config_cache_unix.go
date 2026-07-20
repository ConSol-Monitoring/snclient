//go:build !windows

package snclient

import (
	"fmt"
	"os"
	"syscall"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

func validateHTTPIncludeCacheFileOwner(_ string, stat os.FileInfo) error {
	fileInfoSys, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot inspect cache file owner")
	}

	euid, err := convert.UInt32E(os.Geteuid())
	if err != nil {
		return fmt.Errorf("cannot determine process uid: %w", err)
	}
	if fileInfoSys.Uid != euid {
		return fmt.Errorf("cache file owner uid %d does not match process uid %d", fileInfoSys.Uid, os.Geteuid())
	}

	return nil
}

func getCurrentUserHASH() (hash string, err error) {
	hash, err = utils.Sha256Sum(fmt.Sprintf("%d", os.Geteuid()))
	if err != nil {
		return "", fmt.Errorf("failed to build hash sum: %w", err)
	}

	return hash, nil
}
