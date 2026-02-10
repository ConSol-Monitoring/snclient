//go:build windows

package snclient

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"golang.org/x/sys/windows/registry"
)

type PersistentNetworkDrive struct {
	DriveLetter    string // Only the driveLetter, so not "X:\" but "X"
	ConnectFlags   uint32
	ConnectionType uint32
	ProviderName   string
	ProviderType   uint32
	ProviderFlags  uint32
	DeferFlags     uint32
	RemotePath     string
	UseOptions     []byte
	UserName       string
}

// This function looks into the Windows Registry of the current logged in user for persistent network drives
//
//nolint:gocyclo,funlen // the function is repetitive, but simple
func discoverPersistentNetworkDrives() (savedNetworkDrives []PersistentNetworkDrive, err error) {
	// From the observations, HKEY_CURRENT_USER\Network contains subkeys/subfolders that correspond to drive letters
	// Tracing "C:\Windows\system32\net.exe use" shows that it tries to browse this path to get the current network drives

	hkcuNetwork, err := registry.OpenKey(registry.CURRENT_USER, `Network`, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return []PersistentNetworkDrive{}, fmt.Errorf("Error when browsing into the registry HKCU\\Network: %w", err)
	}

	hkcuNetworkStat, err := hkcuNetwork.Stat()
	if err != nil {
		return []PersistentNetworkDrive{}, fmt.Errorf("Error when getting the stats for HKCU\\Network: %w", err)
	}

	hkcuNetworkSubkeys, err := hkcuNetwork.ReadSubKeyNames(int(hkcuNetworkStat.SubKeyCount))
	if err != nil {
		return []PersistentNetworkDrive{}, fmt.Errorf("Error when listing the subkeys of HCKU\\Network: %w", err)
	}

	networkDrives := make([]PersistentNetworkDrive, 0)

	for _, hkcuNetworkSubkey := range hkcuNetworkSubkeys {
		// HKCU\Network might contain other subkeys that arent drive letters
		if len(hkcuNetworkSubkey) > 1 {
			log.Debugf("Subkey under HKCU\\Network has more than 1 character, cannot be a drive: %s", hkcuNetworkSubkey)

			continue
		}
		firstRune, _ := utf8.DecodeRuneInString(hkcuNetworkSubkey)
		if firstRune == utf8.RuneError {
			log.Debugf("Error when getting the first rune of %s", hkcuNetworkSubkey)
		}
		if !unicode.IsLetter(firstRune) {
			log.Debugf("Subkey under HKCU\\Network is not a letter, cannot be a drive: %s", hkcuNetworkSubkey)

			continue
		}

		networkDrive := PersistentNetworkDrive{}
		networkDrive.DriveLetter = hkcuNetworkSubkey

		driveKey, err := registry.OpenKey(hkcuNetwork, networkDrive.DriveLetter, registry.QUERY_VALUE)
		if err != nil {
			log.Debugf("Error when getting the registry key for drive %s: %s", networkDrive.DriveLetter, err.Error())

			continue
		}

		// net use looked for these keys
		// RemotePath,UserName,ProviderType,ConnectFlags,ProviderFlags,DeferFlags,ConnectionType

		valueStr, valtype, err := driveKey.GetStringValue("RemotePath")
		switch {
		case err == nil:
			networkDrive.RemotePath = valueStr
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a RemotePath", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a RemotePath value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		valueStr, valtype, err = driveKey.GetStringValue("UserName")
		switch {
		case err == nil:
			networkDrive.UserName = valueStr
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a UserName", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a UserName value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		valueUInt64, valtype, err := driveKey.GetIntegerValue("ProviderType")
		switch {
		case err == nil:
			networkDrive.ProviderType = convert.UInt32(valueUInt64)
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a ProviderType", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a ProviderType value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("ConnectFlags")
		switch {
		case err == nil:
			networkDrive.ConnectFlags = convert.UInt32(valueUInt64)
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a ConnectFlags", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a ConnectFlags value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("ProviderFlags")
		switch {
		case err == nil:
			networkDrive.ProviderFlags = convert.UInt32(valueUInt64)
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a ProviderFlags", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a ProviderFlags value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("DeferFlags")
		switch {
		case err == nil:
			networkDrive.DeferFlags = convert.UInt32(valueUInt64)
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a DeferFlags", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a DeferFlags value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("ConnectionType")
		switch {
		case err == nil:
			networkDrive.ConnectionType = convert.UInt32(valueUInt64)
		case errors.Is(err, registry.ErrNotExist):
			log.Debugf("Network Drive %s does not have a ConnectionType", networkDrive.DriveLetter)
		case errors.Is(err, registry.ErrUnexpectedType):
			log.Debugf("Network Drive %s has a ConnectionType value, but the getter function used a wrong type, correct type is %d", networkDrive.DriveLetter, valtype)
		}

		networkDrives = append(networkDrives, networkDrive)
	}

	return networkDrives, nil
}
