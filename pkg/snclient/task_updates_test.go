package snclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSignatureConfig(t *testing.T) {
	handler, ok := NewUpdateHandler().(*UpdateHandler)
	require.True(t, ok)
	assert.True(t, handler.verifySignature)

	section := NewConfig(true).Section("/settings/updates")
	section.Set("verify signature", "false")
	require.NoError(t, handler.setConfig(section))
	assert.False(t, handler.verifySignature)

	section.Set("verify signature", "invalid")
	err := handler.setConfig(section)
	assert.ErrorContains(t, err, "verify signature")
}

func TestGithubReleaseSignaturePairing(t *testing.T) {
	assetName := testUpdateAssetName()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"tag_name":"v1.2.3","assets":[`+
			`{"name":%q,"browser_download_url":%q},`+
			`{"name":%q,"browser_download_url":%q}`+
			`]}]`, assetName, "https://example.test/"+assetName,
			assetName+".sig", "https://example.test/"+assetName+".sig")
	}))
	defer server.Close()

	handler := testUpdateHandler()
	updates, err := handler.checkUpdateGithubRelease(context.Background(), server.URL, "stable", false)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "https://example.test/"+assetName+".sig", updates[0].signatureURL)
}

func TestGithubActionsSignaturePairing(t *testing.T) {
	assetName := testUpdateAssetName()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"artifacts":[`+
			`{"name":%q,"archive_download_url":%q,"workflow_run":{"id":42,"head_branch":"main"}},`+
			`{"name":%q,"archive_download_url":%q,"workflow_run":{"id":42,"head_branch":"main"}}`+
			`]}`, assetName, "https://example.test/package",
			assetName+".sig", "https://example.test/signature")
	}))
	defer server.Close()

	handler := testUpdateHandler()
	handler.snc.config.Section("/settings/updates/channel/dev").Set("github token", "test-token")
	updates, err := handler.checkUpdateGithubActions(context.Background(), server.URL, "dev")
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "https://example.test/signature", updates[0].signatureURL)
	assert.True(t, updates[0].artifactArchives)
}

func TestDownloadUpdateRequiresSignature(t *testing.T) {
	source := filepath.Join(t.TempDir(), "snclient.rpm")
	require.NoError(t, os.WriteFile(source, []byte("unsigned package"), 0o600))

	originalExecutable := GlobalMacros["exe-full"]
	originalExtension := GlobalMacros["file-ext"]
	GlobalMacros["exe-full"] = filepath.Join(t.TempDir(), "snclient")
	GlobalMacros["file-ext"] = ""
	t.Cleanup(func() {
		GlobalMacros["exe-full"] = originalExecutable
		GlobalMacros["file-ext"] = originalExtension
	})

	handler := testUpdateHandler()
	_, err := handler.downloadUpdate(context.Background(), &updatesAvailable{
		channel: "stable",
		url:     "file://" + source,
	})
	require.ErrorContains(t, err, "signature is missing")

	handler.verifySignature = false
	_, err = handler.downloadUpdate(context.Background(), &updatesAvailable{
		channel: "stable",
		url:     "file://" + source,
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "signature is missing")
}

func testUpdateHandler() *UpdateHandler {
	return &UpdateHandler{
		snc:             NewAgentSimple(&AgentFlags{}),
		verifySignature: true,
		httpOptions: &HTTPClientOptions{
			tlsConfig:  &tls.Config{MinVersion: tls.VersionTLS12},
			reqTimeout: 5,
		},
		urlCache: make(map[string]cachedURLVersion),
	}
}

func testUpdateAssetName() string {
	osName := runtime.GOOS
	extension := "rpm"
	if runtime.GOOS == "windows" {
		extension = "msi"
	}
	if runtime.GOOS == "darwin" {
		osName = "osx"
		extension = "pkg"
	}

	return fmt.Sprintf("snclient-1.2.3-%s-%s.%s", osName, pkgArch(runtime.GOARCH), extension)
}
