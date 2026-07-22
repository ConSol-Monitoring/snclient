package snclient

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExporterExporterSkipsUnparsableFiles(t *testing.T) {
	modulesDir := t.TempDir()

	goodYAML := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "some_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "good_module.yaml"),
		[]byte(goodYAML), 0o600,
	))

	badYAMLSyntax := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "bad_metrics.prom") + `
  indentation_error_here
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "bad_syntax.yaml"),
		[]byte(badYAMLSyntax), 0o600,
	))

	duplicateYAML := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "dup_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "good_module.yml"),
		[]byte(duplicateYAML), 0o600,
	))

	unknownFieldYAML := `
method: file
unknown_field: true
file:
  path: ` + filepath.Join(modulesDir, "unknown_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "bad_unknown.yaml"),
		[]byte(unknownFieldYAML), 0o600,
	))

	noMethodYAML := `
file:
  path: ` + filepath.Join(modulesDir, "nomethod_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "bad_nomethod.yaml"),
		[]byte(noMethodYAML), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45670
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45670/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	assert.Containsf(t, string(body), "good_module", "only the valid module should be listed")
	assert.NotContainsf(t, string(body), "bad_syntax", "bad syntax module should be skipped")
	assert.NotContainsf(t, string(body), "bad_unknown", "unknown field module should be skipped")
	assert.NotContainsf(t, string(body), "bad_nomethod", "unknown method module should be skipped")
	assert.NotContainsf(t, string(body), "good_module.yml", "duplicate named module should be skipped")
}

func TestExporterExporterAllowedMethods(t *testing.T) {
	modulesDir := t.TempDir()

	fileModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "allowed_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "file_module.yaml"),
		[]byte(fileModule), 0o600,
	))

	httpModule := `
method: http
http:
  port: 9090
  path: /metrics
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "http_module.yaml"),
		[]byte(httpModule), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45671
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
require password = false
allowed methods = file
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45671/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	assert.Containsf(t, string(body), "file_module", "file method module should be listed")
	assert.NotContainsf(t, string(body), "http_module", "http method module should be excluded by allowed methods")
}

func TestExporterExporterAllowedMethodsMulti(t *testing.T) {
	modulesDir := t.TempDir()

	fileModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "file_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "file_module.yaml"),
		[]byte(fileModule), 0o600,
	))

	httpModule := `
method: http
http:
  port: 9090
  path: /metrics
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "http_module.yaml"),
		[]byte(httpModule), 0o600,
	))

	execModule := `
method: exec
exec:
  command: /usr/bin/prometheus-foo
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "exec_module.yaml"),
		[]byte(execModule), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45676
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
require password = false
allowed methods = file,http
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45676/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	assert.Containsf(t, string(body), "file_module", "file method module should be listed")
	assert.Containsf(t, string(body), "http_module", "http method module should be listed")
	assert.NotContainsf(t, string(body), "exec_module", "exec method module should be excluded by allowed methods")
}

func TestExporterExporterAllFilesFailing(t *testing.T) {
	modulesDir := t.TempDir()

	badYAML := `
method: file
unknown: invalid
  indentation: broken
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "bad1.yaml"),
		[]byte(badYAML), 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "bad2.yaml"),
		[]byte(badYAML), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45672
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45672/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	assert.Containsf(t, string(body), "<h2>Exporters:</h2>", "agent should start even with all config files failing")
	assert.NotContainsf(t, string(body), "bad1", "no module should be listed when all files fail")
	assert.NotContainsf(t, string(body), "bad2", "no module should be listed when all files fail")
}

func TestExporterExporterListEndpointWithPrefix(t *testing.T) {
	modulesDir := t.TempDir()

	fileModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "prefixed_metrics.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "prefixed_module.yaml"),
		[]byte(fileModule), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45673
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /hello
modules dir = ` + modulesDir + `
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45673/hello/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	assert.Containsf(t, string(body), "/hello/proxy?module=prefixed_module", "HTML href should contain the url prefix")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		"http://127.0.0.1:45673/hello/list", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	jsonRes, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	jsonBody, err := io.ReadAll(jsonRes.Body)
	require.NoError(t, err)
	jsonRes.Body.Close()

	assert.Containsf(t, string(jsonBody), "/hello/proxy?module=prefixed_module",
		"JSON output should contain the prefixed proxy URL")
	assert.Containsf(t, string(jsonBody), "prefixed_module",
		"JSON output should contain the module name")
}

func TestExporterExporterConfigDirectoryReload(t *testing.T) {
	modulesDir := t.TempDir()

	initialModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "initial.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "initial.yaml"),
		[]byte(initialModule), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45674
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
modules dir watcher = true
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45674/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	assert.Containsf(t, string(body), "initial", "initial module should be listed")

	time.Sleep(7 * time.Second)

	newModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "reloaded.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "reloaded.yaml"),
		[]byte(newModule), 0o600,
	))

	time.Sleep(3 * time.Second)

	res = waitForStatusOK(t, "http://127.0.0.1:45674/list")
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	assert.Containsf(t, string(body), "reloaded", "newly added module should be listed after reload")
	assert.Containsf(t, string(body), "initial", "original module should still be listed after reload")
}

func TestExporterExporterDefaults(t *testing.T) {
	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45675
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45675/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()

	assert.Containsf(t, string(body), "<h2>Exporters:</h2>", "list endpoint should work with default prefix")
}

func TestExporterExporterConfigDirectoryNoReloadWhenWatcherDisabled(t *testing.T) {
	modulesDir := t.TempDir()

	initialModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "static.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "static.yaml"),
		[]byte(initialModule), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45677
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
modules dir watcher = false
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45677/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	assert.Containsf(t, string(body), "static", "initial module should be listed")

	newModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "not_picked_up.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "not_picked_up.yaml"),
		[]byte(newModule), 0o600,
	))

	time.Sleep(7 * time.Second)

	res = waitForStatusOK(t, "http://127.0.0.1:45677/list")
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	assert.Containsf(t, string(body), "static", "original module should still be listed")
	assert.NotContainsf(t, string(body), "not_picked_up", "new module should NOT be listed when watcher is disabled")
}

func TestExporterExporterFileWatcherIgnoresNonYaml(t *testing.T) {
	modulesDir := t.TempDir()

	initialModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "first.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "first.yaml"),
		[]byte(initialModule), 0o600,
	))

	config := `
[/modules]
WEBServer = enabled
ExporterExporterServer = enabled

[/settings/WEB/server]
port = 45678
use ssl = false
require password = false

[/settings/ExporterExporter/server]
port = ${/settings/WEB/server/port}
use ssl = ${/settings/WEB/server/use ssl}
url prefix = /
modules dir = ` + modulesDir + `
modules dir watcher = true
require password = false
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := waitForStatusOK(t, "http://127.0.0.1:45678/list")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	assert.Containsf(t, string(body), "first", "initial yaml module should be listed")

	time.Sleep(6 * time.Second)

	yamlModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "picked_up.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "picked_up.yml"),
		[]byte(yamlModule), 0o600,
	))

	txtFile := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "ignored.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "ignored.txt"),
		[]byte(txtFile), 0o600,
	))

	time.Sleep(3 * time.Second)

	res = waitForStatusOK(t, "http://127.0.0.1:45678/list")
	body, err = io.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	assert.Containsf(t, string(body), "first", "original module should still be listed")
	assert.Containsf(t, string(body), "picked_up", "yml file should be picked up by watcher")
	assert.NotContainsf(t, string(body), "ignored", "non-yml/yaML file should be ignored by watcher")
}

func waitForStatusOK(t *testing.T, url string) *http.Response {
	t.Helper()

	var lastErr error
	for range 300 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
		require.NoErrorf(t, err, "request created")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)

			continue
		}

		if res.StatusCode == http.StatusOK {
			return res
		}
		res.Body.Close()
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s: %v", url, lastErr)

	return nil
}
