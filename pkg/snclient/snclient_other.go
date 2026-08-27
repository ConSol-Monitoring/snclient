//go:build !linux

package snclient

//nolint:unused,nolintlint // only used on linux actually
func clearInheritableCaps() error {
	return nil
}

//nolint:unused,nolintlint // only used on linux actually
func prepareCapsForExec() error {
	return nil
}
