//go:build !linux

package snclient

func getInode(_ string) uint64 {
	return 0
}
