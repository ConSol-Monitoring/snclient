package snclient

import (
	"strings"
)

// qualifyUsername adds the current users domain to the username if it does not already contain a domain or UPN.
func qualifyUsername(username, domain string) string {
	if strings.Contains(username, "\\") || strings.Contains(username, "@") {
		return username
	}
	if domain == "" {
		return username
	}

	return domain + "\\" + username
}

// shareTargetFromUNCPath returns the server name of a UNC path, e.g. \\server\share -> server
func shareTargetFromUNCPath(path string) string {
	normalized := strings.ReplaceAll(path, "/", "\\")
	parts := strings.Split(normalized, "\\")
	// UNC paths look like \\server\share, split parts: ["", "", "server", "share"]
	if len(parts) >= 3 && parts[0] == "" && parts[1] == "" {
		return parts[2]
	}

	return ""
}

// normalizeCredentialTargetFromUNCPath turns a UNC path into a plain server name, the target name format expected by the SMB redirector.
func normalizeCredentialTargetFromUNCPath(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "\\\\") || strings.HasPrefix(target, "//") {
		return shareTargetFromUNCPath(target)
	}

	return target
}

// shareRoot returns the share root of a UNC path, e.g. \\server\share for \\server\share\folder\file.
func shareRoot(path string) string {
	parts := strings.Split(path, "\\")
	// UNC paths are in the form \\server\share\...
	// share root is the 4th element
	if len(parts) < 4 {
		return path
	}

	return strings.Join(parts[:4], "\\")
}

// isNetworkSharePath returns if the given path looks like an UNC path.
// Example 1: \\FileServer01\PublicDocs
// Example 2: \\BackupServer\Data\Archive\2025-01-14.zip
// Example 3: \\192.168.1.50\SharedData\Images
// Modern programs also generally accept forward slash definitions,
// e.g. //192.168.1.50/Shareddata/Images
func isNetworkSharePath(path string) bool {
	if len(path) < 2 {
		return false
	}

	if !strings.HasPrefix(path, "\\\\") && !strings.HasPrefix(path, "//") {
		return false
	}

	return true
}
