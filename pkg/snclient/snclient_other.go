//go:build !linux

package snclient

//nolint:unused,nolintlint // only used on linux actually
func clearInheritableCaps() error {
	return nil
}
