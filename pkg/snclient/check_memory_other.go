//go:build !windows

package snclient

func (l *CheckMemory) virtualMemory() (total, avail uint64, err error) {
	return 0, 0, err
}
