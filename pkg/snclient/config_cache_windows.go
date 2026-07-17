//go:build windows

package snclient

import "os"

func validateHTTPIncludeCacheFileOwner(_ os.FileInfo) error {
	return nil
}
