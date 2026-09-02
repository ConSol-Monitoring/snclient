package snclient

import (
	"errors"
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

// errSessionCredentialConflict is returned when a connection to the target server
// already exists with a different user name.
var errSessionCredentialConflict = errors.New("a connection to the server already exists with different credentials")

// Credential describes a single entry in the [/settings/credentials] section.
type Credential struct {
	Type        string
	Target      string
	Username    string
	Password    string
	PasswordSet bool
	Strategy    string
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
			cred.PasswordSet = true
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
