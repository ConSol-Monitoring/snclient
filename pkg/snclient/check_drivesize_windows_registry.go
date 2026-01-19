package snclient

import (
	"errors"
	"fmt"
	"unicode"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"golang.org/x/sys/windows/registry"
)

type SavedNetworkDrive struct {
	DriveLetter    string
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

// This function looks into the Windows Registery of the current logged in user for persistent network drives
func discoverPersistentNetworkDrives() (savedNetworkDrives []SavedNetworkDrive, err error) {
	// From the observations, HKEY_CURRENT_USER\Network contains subkeys/subfolders that correspond to drive letters
	// Tracing "C:\Windows\system32\net.exe use" shows that it tries to browse this path to get the current network drives

	hkcuNetwork, err := registry.OpenKey(registry.CURRENT_USER, `Network`, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return []SavedNetworkDrive{}, fmt.Errorf("Error when browsing into the registery HKCU\\Network: %w", err)
	}

	hkcuNetworkStat, err := hkcuNetwork.Stat()
	if err != nil {
		return []SavedNetworkDrive{}, fmt.Errorf("Error when getting the stats for HKCU\\Network: %w", err)
	}

	hkcuNetworkSubkeys, err := hkcuNetwork.ReadSubKeyNames(int(hkcuNetworkStat.SubKeyCount))
	if err != nil {
		return []SavedNetworkDrive{}, fmt.Errorf("Error when listing the subkeys of HCKU\\Network: %w", err)
	}

	networkDrives := make([]SavedNetworkDrive, 0)

	for _, hkcuNetworkSubkey := range hkcuNetworkSubkeys {
		// HKCU\Network might contain other subkeys that arent drive letters
		if len(hkcuNetworkSubkey) > 1 {
			log.Debugf("Subkey under HKCU\\Network has more than 1 character, cannot be a drive: %s", hkcuNetworkSubkey)
			continue
		}
		if !unicode.IsLetter(([]rune)(hkcuNetworkSubkey)[0]) {
			log.Debugf("Subkey under HKCU\\Network is not a letter, cannot be a drive: %s", hkcuNetworkSubkey)
			continue
		}

		networkDrive := SavedNetworkDrive{}
		networkDrive.DriveLetter = hkcuNetworkSubkey

		driveKey, err := registry.OpenKey(hkcuNetwork, networkDrive.DriveLetter, registry.QUERY_VALUE)
		if err != nil {
			log.Debugf("Error when getting the registery key for drive %s: %e", networkDrive.DriveLetter, err.Error())
			continue
		}

		// net use looked for these keys
		// RemotePath,UserName,ProviderType,ConnectFlags,ProviderFlags,DeferFlags,ConnectionType

		valueStr, valtype, err := driveKey.GetStringValue("RemotePath")
		if err == nil {
			networkDrive.RemotePath = valueStr
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a RemotePath", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a RemotePath value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		valueStr, valtype, err = driveKey.GetStringValue("UserName")
		if err == nil {
			networkDrive.UserName = valueStr
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a UserName", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a UserName value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		valueUInt64, valtype, err := driveKey.GetIntegerValue("ProviderType")
		if err == nil {
			networkDrive.ProviderType = convert.UInt32(valueUInt64)
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a ProviderType", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a ProviderType value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("ConnectFlags")
		if err == nil {
			networkDrive.ConnectFlags = convert.UInt32(valueUInt64)
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a ConnectFlags", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a ConnectFlags value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("ProviderFlags")
		if err == nil {
			networkDrive.ProviderFlags = convert.UInt32(valueUInt64)
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a ProviderFlags", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a ProviderFlags value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("DeferFlags")
		if err == nil {
			networkDrive.DeferFlags = convert.UInt32(valueUInt64)
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a DeferFlags", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a DeferFlags value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		valueUInt64, valtype, err = driveKey.GetIntegerValue("ConnectionType")
		if err == nil {
			networkDrive.ConnectionType = convert.UInt32(valueUInt64)
		} else if errors.Is(err, registry.ErrNotExist) {
			log.Debugf("Network Drive %s does not have a ConnectionType", networkDrive.DriveLetter)
		} else if errors.Is(err, registry.ErrUnexpectedType) {
			log.Debugf("Network Drive %s has a ConnectionType value, but the getter function used a wrong type, correct type is %s", valtype)
		}

		networkDrives = append(networkDrives, networkDrive)
	}

	return networkDrives, nil
}
