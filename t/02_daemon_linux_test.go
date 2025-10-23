package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonRequestsLinux(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `./snclient.ini`, localDaemonINI)

	startBackgroundDaemon(t)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", localDaemonPort)

	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-k", "--header", "password:" + localDaemonPassword, baseURL + "/query/check_echo?%20"},
		Like: []string{`"check_echo"`, `"OK"`},
		Exit: 0,
	})

	stopBackgroundDaemon(t)
	os.Remove("snclient.ini")
	os.Remove("test.crt")
	os.Remove("test.key")
}

func TestErrorBetweenSavingAndSigning(t *testing.T) {
	_, baseURL, _, cleanUp := daemonInit(t, "")
	defer os.Remove("test.crt")
	defer os.Remove("test.key")
	defer os.Remove("test.csr")

	rawPostData := map[string]any{
		"Country":            "DE",
		"State":              "Bavaria",
		"Locality":           "Earth",
		"Organization":       "snclient",
		"OrganizationalUnit": "IT",
		"HostName":           "Root CA SNClient",
		"NewKey":             true,
		"KeyLength":          1024,
	}

	postData, err := json.Marshal(rawPostData)
	require.NoErrorf(t, err, "post data json encoded")

	// Create  Temp Server Certs
	runCmd(t, &cmd{
		Cmd:     "make",
		Args:    []string{"testca"},
		Like:    []string{"certificate request ok"},
		ErrLike: []string{".*"},
	})
	defer runCmd(t, &cmd{
		Cmd:  "make",
		Args: []string{"clean-testca"},
		Like: []string{"dist"},
	})

	commandResult := runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", "-s", "-d", string(postData), baseURL + "/api/v1/admin/csr"},
		Dir:  ".",
		Like: []string{"CERTIFICATE REQUEST"},
	})
	err = os.WriteFile("test.csr", []byte(commandResult.Stdout), 0o600)
	if err != nil {
		t.Fatalf("could not save certificate signing requests")
	}

	runCmd(t, &cmd{
		Cmd:     "openssl",
		Args:    []string{"x509", "-req", "-in=test.csr", "-CA=dist/cacert.pem", "-CAkey=dist/ca.key", "-out=server.crt", "-days=365"},
		ErrLike: []string{".*"},
	})
	runCmd(t, &cmd{
		Cmd:     "openssl",
		Args:    []string{"x509", "-req", "-in=test.csr", "-CA=dist/cacert.pem", "-CAkey=dist/ca.key", "-noout", "-subject"},
		ErrLike: []string{".*"},
	})
	defer os.Remove("server.crt")

	keyBak, _ := os.ReadFile("test.key.tmp")
	newCert, _ := os.ReadFile("server.crt")

	// restart client
	cleanUp()
	_, baseURL, _, cleanUp = daemonInit(t, "")
	defer cleanUp()

	postData, err = json.Marshal(map[string]interface{}{
		"Reload":   true,
		"CertData": base64.StdEncoding.EncodeToString(newCert),
		"KeyData":  "",
	})
	require.NoErrorf(t, err, "post data json encoded")

	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", "-s", "-d", string(postData), baseURL + "/api/v1/admin/certs/replace"},
		Like: []string{`{"success":true}`},
	})

	// Check if new private Key matches the on we got from the csr Endpoint
	key, _ := os.ReadFile("test.key")
	assert.Equalf(t, string(keyBak), string(key), "private keys do not match")

	_, err = os.ReadFile("test.key.tmp")
	if err == nil {
		t.Fatalf("temporary key file was not removed")
	}

	// request csr with challenge password
	rawPostData["ChallengePassword"] = "test123"
	postData, err = json.Marshal(rawPostData)
	require.NoErrorf(t, err, "post data json encoded")
	commandResult = runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", "-s", "-d", string(postData), baseURL + "/api/v1/admin/csr"},
		Dir:  ".",
		Like: []string{"CERTIFICATE REQUEST"},
	})

	err = os.WriteFile("test.csr", []byte(commandResult.Stdout), 0o600)
	if err != nil {
		t.Fatalf("could not save certificate signing requests")
	}

	runCmd(t, &cmd{
		Cmd:  "openssl",
		Args: []string{"req", "-in", "test.csr", "-noout", "-text"},
		Like: []string{"challengePassword", "test123", "sha256WithRSAEncryption"},
	})
}

func TestHSTSHeaderEnabled(t *testing.T) {
	runCmd(t, &cmd{
		Cmd:     "make",
		Args:    []string{"testca"},
		Like:    []string{"certificate request ok"},
		ErrLike: []string{".*"},
	})
	defer runCmd(t, &cmd{
		Cmd:  "make",
		Args: []string{"clean-testca"},
		Like: []string{"dist"},
	})

	_, baseURL, _, cleanUp := daemonInit(t, localDaemonINI+`
[/settings/default]
certificate = dist/server.crt
certificate key = dist/server.key
use hsts header = enabled

[/settings/WEB/server]
use ssl = enabled

[/settings/WEBAdmin/server]
use ssl = enabled
`)
	defer cleanUp()

	assert.NotEmpty(t, baseURL)
	baseURL = fmt.Sprintf("https://127.0.0.1:%d", localDaemonPort)

	runCmd(t, &cmd{
		Cmd:     "curl",
		Args:    []string{"-s", "-v", "-k", "-u", "user:" + localDaemonAdminPassword, "-k", baseURL + "/index.html"},
		Like:    []string{`snclient working`},
		ErrLike: []string{`Strict-Transport-Security`},
	})
}
