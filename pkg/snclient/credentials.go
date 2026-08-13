package snclient

import (
	"sort"
	"strings"
)

const (
	// CredentialTypeWindowsShare stores a domain password credential in the Windows
	// Credential Manager that is automatically used by the SMB redirector when
	// connecting to the given target server.
	CredentialTypeWindowsShare = "windows-share"

	// CredentialStrategyOnStart loads the credential once at agent startup.
	// It stays for the lifetime of the agent logon session and is gone after a reboot.
	CredentialStrategyOnStart = "on-start"

	// CredentialStrategyOnDemand loads the credential right before it is needed
	// and removes it again as soon as the check finished.
	CredentialStrategyOnDemand = "on-demand"
)

// Credential describes a single entry in the [/settings/credentials] section.
type Credential struct {
	Type     string
	Target   string
	Username string
	Password string
	Strategy string
}

// parseCredentials reads the [/settings/credentials] section and returns all entries.
// Invalid or unsupported entries are skipped with a warning, so one broken entry does not break the whole configuration.
func parseCredentials(config *Config) (credentials []Credential) {
	sections := config.SectionsByPrefix("/settings/credentials/")
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)

	domain := currentUserDomain()

	for _, name := range names {
		section := sections[name]

		cred := Credential{
			Type:     CredentialTypeWindowsShare,
			Strategy: CredentialStrategyOnDemand,
		}
		if val, ok := section.GetString("type"); ok && val != "" {
			cred.Type = strings.ToLower(strings.TrimSpace(val))
		}
		if val, ok := section.GetString("target"); ok {
			cred.Target = strings.TrimSpace(val)
		}
		if val, ok := section.GetString("username"); ok {
			cred.Username = strings.TrimSpace(val)
		}
		if val, ok := section.GetString("password"); ok {
			cred.Password = val
		}
		if val, ok := section.GetString("strategy"); ok && val != "" {
			cred.Strategy = strings.ToLower(strings.TrimSpace(val))
		}

		switch cred.Type {
		case CredentialTypeWindowsShare:
		default:
			log.Warnf("credentials: unsupported type %q in %s, only %q is supported, skipping", cred.Type, name, CredentialTypeWindowsShare)

			continue
		}

		if cred.Target == "" {
			log.Warnf("credentials: missing target in %s, skipping", name)

			continue
		}

		switch cred.Strategy {
		case CredentialStrategyOnStart, CredentialStrategyOnDemand:
		default:
			log.Warnf("credentials: unsupported strategy %q in %s, skipping", cred.Strategy, name)

			continue
		}

		if cred.Username == "" {
			log.Warnf("credentials: missing username in %s, skipping", name)

			continue
		}

		cred.Username = qualifyUsername(cred.Username, domain)

		credentials = append(credentials, cred)
	}

	return credentials
}

// applyCredentialsOnStart loads all credentials configured with the on-start strategy.
// It is called once at agent startup and again on config reloads.
func applyCredentialsOnStart(config *Config) {
	for _, cred := range parseCredentials(config) {
		if cred.Strategy != CredentialStrategyOnStart {
			continue
		}

		if cred.Type == CredentialTypeWindowsShare {
			if err := addShareCredential(&cred); err != nil {
				log.Errorf("credentials: failed to add on-start credential for %s: %s", cred.Target, err.Error())

				continue
			}
			log.Debugf("credentials: added on-start windows share credential for %s", cred.Target)
		}
	}
}

// findOnDemandCredential returns the on-demand credential matching the given target.
func findOnDemandCredential(config *Config, target string) (Credential, bool) {
	for _, cred := range parseCredentials(config) {
		if cred.Strategy != CredentialStrategyOnDemand {
			continue
		}
		if strings.EqualFold(normalizeCredentialTargetFromUNCPath(cred.Target), normalizeCredentialTargetFromUNCPath(target)) {
			return cred, true
		}
	}

	return Credential{}, false
}

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
