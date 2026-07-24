package snclient

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// speed up testing
	ExporterExporterDebounceDuration = 200 * time.Millisecond
}

func TestExporterExporterSkipsUnparsableFiles(t *testing.T) {
	logBuffer := bytes.Buffer{}
	disableLogsTemporarilyToBuffer(&logBuffer)
	defer restoreLogTarget()

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

	body := waitForStatusOK(t, "http://127.0.0.1:45670/list")
	assert.Containsf(t, body, "good_module", "only the valid module should be listed")
	assert.NotContainsf(t, body, "bad_syntax", "bad syntax module should be skipped")
	assert.NotContainsf(t, body, "bad_unknown", "unknown field module should be skipped")
	assert.NotContainsf(t, body, "bad_nomethod", "unknown method module should be skipped")
	assert.NotContainsf(t, body, "good_module.yml", "duplicate named module should be skipped")

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "bad_nomethod.yaml, unknown module method")
	assert.Contains(t, logOutput, "bad_syntax.yaml, yaml.Unmarshal: yaml: line 5:")
	assert.Contains(t, logOutput, "bad_unknown.yaml, unknown module configuration")
}

func TestExporterExporterAllowedMethods(t *testing.T) {
	logBuffer := bytes.Buffer{}
	disableLogsTemporarilyToBuffer(&logBuffer)
	defer restoreLogTarget()

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

	body := waitForStatusOK(t, "http://127.0.0.1:45671/list")
	assert.Containsf(t, body, "file_module", "file method module should be listed")
	assert.NotContainsf(t, body, "http_module", "http method module should be excluded by allowed methods")

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, `does not contain the module config method: 'http'`)
}

func TestExporterExporterAllowedMethodsMulti(t *testing.T) {
	logBuffer := bytes.Buffer{}
	disableLogsTemporarilyToBuffer(&logBuffer)
	defer restoreLogTarget()

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

	body := waitForStatusOK(t, "http://127.0.0.1:45676/list")
	assert.Containsf(t, body, "file_module", "file method module should be listed")
	assert.Containsf(t, body, "http_module", "http method module should be listed")
	assert.NotContainsf(t, body, "exec_module", "exec method module should be excluded by allowed methods")

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, `does not contain the module config method: 'exec'`)
}

func TestExporterExporterAllFilesFailing(t *testing.T) {
	logBuffer := bytes.Buffer{}
	disableLogsTemporarilyToBuffer(&logBuffer)
	defer restoreLogTarget()

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

	body := waitForStatusOK(t, "http://127.0.0.1:45672/list")
	assert.Containsf(t, body, "<h2>Exporters:</h2>", "agent should start even with all config files failing")
	assert.NotContainsf(t, body, "bad1", "no module should be listed when all files fail")
	assert.NotContainsf(t, body, "bad2", "no module should be listed when all files fail")

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, `bad1.yaml, yaml.Unmarshal: yaml: line 4: mapping values are not allowed in this context`)
	assert.Contains(t, logOutput, `bad2.yaml, yaml.Unmarshal: yaml: line 4: mapping values are not allowed in this context`)
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

	body := waitForStatusOK(t, "http://127.0.0.1:45673/hello/list")
	assert.Containsf(t, body, "/hello/proxy?module=prefixed_module", "HTML href should contain the url prefix")

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

	body := waitForStatusOK(t, "http://127.0.0.1:45674/list")
	assert.Containsf(t, body, "initial", "initial module should be listed")
	assert.NotContainsf(t, body, "reloaded", "newly module should not be listed before reload")

	newModule := `
method: file
file:
  path: ` + filepath.Join(modulesDir, "reloaded.prom") + `
`
	require.NoError(t, os.WriteFile(
		filepath.Join(modulesDir, "reloaded.yaml"),
		[]byte(newModule), 0o600,
	))

	body = waitForStatusOKText(t, "http://127.0.0.1:45674/list", "reloaded")
	assert.Containsf(t, body, "reloaded", "newly added module should be listed after reload")
	assert.Containsf(t, body, "initial", "original module should still be listed after reload")
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

	body := waitForStatusOK(t, "http://127.0.0.1:45675/list")
	assert.Containsf(t, body, "<h2>Exporters:</h2>", "list endpoint should work with default prefix")
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

	body := waitForStatusOK(t, "http://127.0.0.1:45678/list")
	assert.Containsf(t, body, "first", "initial yaml module should be listed")
	assert.NotContainsf(t, body, "picked_up", "yml file should not yet be picked up")

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

	body = waitForStatusOKText(t, "http://127.0.0.1:45678/list", "picked_up")
	assert.Containsf(t, body, "first", "original module should still be listed")
	assert.Containsf(t, body, "picked_up", "yml file should be picked up by watcher")
	assert.NotContainsf(t, body, "ignored", "non-yml/yaML file should be ignored by watcher")
}

func waitForStatusOK(t *testing.T, url string) string {
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
			body, hErr := io.ReadAll(res.Body)
			require.NoError(t, hErr)
			res.Body.Close()

			return string(body)
		}
		res.Body.Close()
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s: %v", url, lastErr)

	return ""
}

// wait for http ok and given text
func waitForStatusOKText(t *testing.T, url, text string) string {
	t.Helper()
	for range 300 {
		body := waitForStatusOK(t, url)
		if strings.Contains(body, text) {
			return body
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s to contain %q", url, text)

	return ""
}
