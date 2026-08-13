package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualifyUsername(t *testing.T) {
	tests := []struct {
		username string
		domain   string
		want     string
	}{
		{`svc`, `CORP`, `CORP\svc`},
		{`DOMAIN\svc`, `CORP`, `DOMAIN\svc`},
		{`svc@corp.example.com`, `CORP`, `svc@corp.example.com`},
		{`corp.example.com\svc`, `CORP`, `corp.example.com\svc`},
		{`svc`, ``, `svc`},
	}
	for _, test := range tests {
		assert.Equalf(t, test.want, qualifyUsername(test.username, test.domain), "qualifyUsername(%q, %q)", test.username, test.domain)
	}
}

func TestShareTargetFromUNC(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{`\\server\share`, `server`},
		{`\\server\share\folder`, `server`},
		{`\\server\C$`, `server`},
		{`\\192.168.178.21\TestHidden$`, `192.168.178.21`},
		{`//server/share`, `server`},
		{`C:\folder`, ``},
		{`server\share`, ``},
		{``, ``},
	}
	for _, test := range tests {
		assert.Equalf(t, test.want, shareTargetFromUNCPath(test.path), "shareTargetFromUNC(%q)", test.path)
	}
}

func TestIsNetworkSharePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`\\server\share`, true},
		{`//server/share`, true},
		{`\\server`, true},
		{`C:\folder`, false},
		{`C:`, false},
		{`/`, false},
		{``, false},
	}
	for _, test := range tests {
		assert.Equalf(t, test.want, isNetworkSharePath(test.path), "isNetworkSharePath(%q)", test.path)
	}
}

func TestNormalizeCredentialTarget(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{`\\server\share`, `server`},
		{`\\server\C$\folder`, `server`},
		{`//server/share`, `server`},
		{`server`, `server`},
		{`server2`, `server2`},
		{``, ``},
	}
	for _, test := range tests {
		assert.Equalf(t, test.want, normalizeCredentialTargetFromUNCPath(test.target), "normalizeCredentialTarget(%q)", test.target)
	}
}

func TestParseCredentials(t *testing.T) {
	config := NewConfig(true)
	parent := config.Section("/settings/credentials")
	parent.Set("strategy", "on-start")

	// explicit entry, all keys set
	share1 := config.Section("/settings/credentials/share1")
	share1.Set("type", "windows-share")
	share1.Set("target", `\\server1`)
	share1.Set("username", `CORP\svc`)
	share1.Set("password", "secret")
	share1.Set("strategy", "on-demand")

	// type and strategy default to windows-share / parent strategy
	share2 := config.Section("/settings/credentials/share2")
	share2.Set("target", "server2")
	share2.Set("username", "svc@corp.example.com")
	share2.Set("password", "secret2")

	// unsupported type, must be skipped
	share3 := config.Section("/settings/credentials/share3")
	share3.Set("type", "generic")
	share3.Set("target", "server3")
	share3.Set("username", `CORP\svc`)

	// missing username, must be skipped
	share4 := config.Section("/settings/credentials/share4")
	share4.Set("target", "server4")
	share4.Set("password", "secret4")

	// unsupported strategy, must be skipped
	share5 := config.Section("/settings/credentials/share5")
	share5.Set("target", "server5")
	share5.Set("username", `CORP\svc`)
	share5.Set("strategy", "sometimes")

	credentials := parseCredentials(config)
	require.Lenf(t, credentials, 2, "two valid credentials expected")

	assert.Equal(t, CredentialTypeWindowsShare, credentials[0].Type)
	assert.Equal(t, `\\server1`, credentials[0].Target)
	assert.Equal(t, `CORP\svc`, credentials[0].Username)
	assert.Equal(t, "secret", credentials[0].Password)
	assert.Equal(t, CredentialStrategyOnDemand, credentials[0].Strategy)

	assert.Equal(t, CredentialTypeWindowsShare, credentials[1].Type)
	assert.Equal(t, "server2", credentials[1].Target)
	assert.Equal(t, "svc@corp.example.com", credentials[1].Username)
	assert.Equal(t, CredentialStrategyOnStart, credentials[1].Strategy)
}

func TestFindOnDemandCredential(t *testing.T) {
	config := NewConfig(true)
	share := config.Section("/settings/credentials/share1")
	share.Set("target", `\\server1\C$`)
	share.Set("username", `CORP\svc`)
	share.Set("strategy", "on-demand")

	cred, ok := findOnDemandCredential(config, `server1`)
	require.True(t, ok)
	assert.Equal(t, `\\server1\C$`, cred.Target)

	// case-insensitive match
	cred, ok = findOnDemandCredential(config, `SERVER1`)
	require.True(t, ok)
	assert.Equal(t, `\\server1\C$`, cred.Target)

	_, ok = findOnDemandCredential(config, `otherserver`)
	assert.False(t, ok)
}
