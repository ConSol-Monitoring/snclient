package snclient

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPkgDefaults(t *testing.T) {
	cfg := NewConfig(true)
	err := cfg.ParseINIFile("../../packaging/snclient.ini", nil)

	// verify default nasty characters
	nastyChars, _ := cfg.Section("/settings/WEB/server").GetString("nasty characters")
	assert.Equalf(t, DefaultNastyCharacters, nastyChars, "default nasty characters")
	assert.Containsf(t, nastyChars, "\\", "nasty characters contains backslash")

	require.NoErrorf(t, err, "config parsed")
}

func TestConfigBasic(t *testing.T) {
	configText := `
[/test]
Key1 = Value1
Key2 = "Value2"
Key3 = 'Value3'
test = 'C:\Program Files\snclient\snclient.exe' -V
test1 = test1 # test
test2 = test2 ; test
test3 = "test3" "test3"
test4 = "a"
test4 += 'b'
test4 += c
; comment
# also comment
	`
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, "testfile.ini", nil)

	require.NoErrorf(t, err, "config parsed")

	expData := ConfigData{
		"Key1":  "Value1",
		"Key2":  "Value2",
		"Key3":  "Value3",
		"test":  `'C:\Program Files\snclient\snclient.exe' -V`,
		"test1": `test1 # test`,
		"test2": `test2 ; test`,
		"test3": `"test3" "test3"`,
		"test4": "abc",
	}
	assert.Equalf(t, expData, cfg.Section("/test").data, "config parsed")
}

func TestConfigErrorI(t *testing.T) {
	configText := `
[/test]
Key1 = "Value1
	`
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, "testfile.ini", nil)

	require.Errorf(t, err, "config error found")
	require.ErrorContains(t, err, "config error in testfile.ini:3: unclosed quotes")
}

func TestConfigStringParent(t *testing.T) {
	configText := `
[/settings/default]
Key1 = Value1

[/settings/sub1]
Key4 = Value4

[/settings/sub1/default]
Key2 = Value2

[/settings/sub1/other]
Key3 = Value3

	`
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, "testfile.ini", nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/sub1/other")
	val3, _ := section.GetString("Key3")
	assert.Equalf(t, "Value3", val3, "got val3")

	val2, _ := section.GetString("Key2")
	assert.Equalf(t, "Value2", val2, "got val2")

	val1, _ := section.GetString("Key1")
	assert.Equalf(t, "Value1", val1, "got val1")

	val4, _ := section.GetString("Key4")
	assert.Equalf(t, "Value4", val4, "got val4")
}

func TestConfigDefaultPassword(t *testing.T) {
	defaultConfig := `
[/settings/WEB/server]
password = CHANGEME
	`
	customConfig := `
[/settings/default]
password = test
	`

	cfg := NewConfig(false)
	err := cfg.ParseINI(defaultConfig, "default.ini", nil)
	require.NoErrorf(t, err, "default config parsed")

	err = cfg.ParseINI(customConfig, "custom.ini", nil)
	require.NoErrorf(t, err, "custom config parsed")

	section := cfg.Section("/settings/WEB/server")
	val, _ := section.GetString("password")
	assert.Equalf(t, "test", val, "got custom password")
}

func TestConfigIncludeFile(t *testing.T) {
	testDir, _ := os.Getwd()
	configsDir := filepath.Join(testDir, "t", "configs")
	configText := fmt.Sprintf(`
[/settings/NRPE/server]
port = 5666

[/settings/WEB/server]
port = 443
password = supersecret

[/includes]
custom_ini = %s/nrpe_web_ports.ini

	`, configsDir)
	iniFile, _ := os.CreateTemp(t.TempDir(), "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	require.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name(), nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/NRPE/server")
	nrpePort, _ := section.GetString("port")
	assert.Equalf(t, "15666", nrpePort, "got nrpe port")

	section = cfg.Section("/settings/WEB/server")
	webPort, _ := section.GetString("port")
	assert.Equalf(t, "1443", webPort, "got web port")
	webPassword, _ := section.GetString("password")
	assert.Equalf(t, "soopersecret", webPassword, "got web password")
}

func TestConfigIncludeDir(t *testing.T) {
	testDir, _ := os.Getwd()
	configsDir := filepath.Join(testDir, "t", "configs")
	customDir := filepath.Join(testDir, "t", "configs", "custom")
	configText := fmt.Sprintf(`
[/settings/NRPE/server]
port = 5666

[/settings/WEB/server]
port = 443
password = supersecret

[/includes]
custom_ini = %s/nrpe_web_ports.ini
custom_ini_dir = %s

	`, configsDir, customDir)
	iniFile, _ := os.CreateTemp(t.TempDir(), "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	require.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name(), nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/NRPE/server")
	nrpePort, _ := section.GetString("port")
	assert.Equalf(t, "11111", nrpePort, "got nrpe port")

	section = cfg.Section("/settings/WEB/server")
	webPort, _ := section.GetString("port")
	assert.Equalf(t, "84433", webPort, "got web port")
	webPassword, _ := section.GetString("password")
	assert.Equalf(t, "consol123", webPassword, "got web password")
}

func TestConfigIncludeWildcards(t *testing.T) {
	testDir, _ := os.Getwd()
	configsDir := filepath.Join(testDir, "t", "configs")
	customDir := filepath.Join(testDir, "t", "configs", "custom")
	configText := fmt.Sprintf(`
[/settings/NRPE/server]
port = 5666

[/settings/WEB/server]
port = 443
password = supersecret

[/includes]
custom_ini = %s/nrpe_web_ports.ini
custom_ini_dir = %s
custom_ini_wc = %s/nrpe_web_ports_*.ini

	`, configsDir, customDir, configsDir)
	iniFile, _ := os.CreateTemp(t.TempDir(), "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	require.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name(), nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/NRPE/server")
	nrpePort, _ := section.GetString("port")
	assert.Equalf(t, "12345", nrpePort, "got nrpe port")

	section = cfg.Section("/settings/WEB/server")
	webPort, _ := section.GetString("port")
	assert.Equalf(t, "1919", webPort, "got web port")
	webPassword, _ := section.GetString("password")
	assert.Equalf(t, "s00pers3cr3t", webPassword, "got web password")
}

func TestConfigUTF8BOM(t *testing.T) {
	testDir, _ := os.Getwd()
	configDir := filepath.Join(testDir, "t", "configs", "utf8_bom")

	cfg := NewConfig(true)
	err := cfg.ReadINI(configDir+"/snclient.ini", nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/paths")
	exePath, _ := section.GetString("exe-path")
	assert.Equalf(t, "./tmp", exePath, "got path")
}

func TestConfigWrite(t *testing.T) {
	configText := `
; nrpe help
[/settings/NRPE/server]
; port - port description
port = 5666


; web help 1
; web help 2
[/settings/WEB/server]
; port - port description
port = 443

; use ssl - security is important hmmkay
; use ssl = false


[/includes]
; only comment1
; only comment2
`
	if runtime.GOOS == "windows" {
		// assume original config file has windows newlines
		configText = strings.ReplaceAll(configText, "\n", "\r\n")
	}

	cfg := NewConfig(false)
	err := cfg.ParseINI(configText, "test.ini", nil)

	require.NoErrorf(t, err, "parsed ini without error")
	assert.Equalf(t, strings.TrimSpace(configText), strings.TrimSpace(cfg.ToString()), "config did no change")

	changedConfig := `
; nrpe help
[/settings/NRPE/server]
; port - port description
port = 5666


; web help 1
; web help 2
[/settings/WEB/server]
; port - port description
port = 1234

; use ssl - security is important hmmkay
use ssl = enabled


[/includes]
; only comment1
; only comment2
test = ./test.ini
`
	if runtime.GOOS == "windows" {
		// assume original config file has windows newlines
		changedConfig = strings.ReplaceAll(changedConfig, "\n", "\r\n")
	}

	cfg.Section("/settings/WEB/server").Insert("port", "1234")
	cfg.Section("/settings/WEB/server").Insert("use ssl", "enabled")
	cfg.Section("/includes").Insert("test", "./test.ini")

	assert.Equalf(t, strings.TrimSpace(changedConfig), strings.TrimSpace(cfg.ToString()), "config changed correctly")
}

func TestConfigPackaging(t *testing.T) {
	testDir, _ := os.Getwd()
	pkgDir := filepath.Join(testDir, "..", "..", "packaging")
	pkgCfgFile := filepath.Join(pkgDir, "snclient.ini")

	data, err := os.ReadFile(pkgCfgFile)
	require.NoErrorf(t, err, "read ini without error")
	origConfig := strings.TrimSpace(string(data))

	if runtime.GOOS == "windows" {
		// assume original config file has windows newlines
		origConfig = strings.ReplaceAll(origConfig, "\r\n", "\n")
		origConfig = strings.ReplaceAll(origConfig, "\n", "\r\n")
	}

	cfg := NewConfig(false)
	err = cfg.ParseINIFile(pkgCfgFile, nil)

	require.NoErrorf(t, err, "parse ini without error")
	assert.Equalf(t, origConfig, strings.TrimSpace(cfg.ToString()), "default config should not change when opened and saved unchanged")
}

func TestConfigRelativeIncludes(t *testing.T) {
	testDir, _ := os.Getwd()
	pkgDir := filepath.Join(testDir, "t", "configs")
	pkgCfgFile := filepath.Join(pkgDir, "snclient_incl.ini")

	cfg := NewConfig(true)
	err := cfg.ParseINIFile(pkgCfgFile, nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/WEB/server")
	webPort, _ := section.GetString("port")
	assert.Equalf(t, "11122", webPort, "got web port")
	useSSL, _ := section.GetString("use ssl")
	assert.Equalf(t, "false", useSSL, "got use ssl")
	webPassword, _ := section.GetString("password")
	assert.Equalf(t, "INCL02PW", webPassword, "got password")
	modules := cfg.Section("/modules")
	ces, _ := modules.GetString("CheckExternalScripts")
	assert.Equalf(t, "enabled", ces, "got CheckExternalScripts")
}

func TestEmptyConfig(t *testing.T) {
	configText := `; INI
`
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, "testfile.ini", nil)

	require.NoErrorf(t, err, "empty ini parsed")
}

func TestConfigAppend(t *testing.T) {
	testDir, _ := os.Getwd()
	pkgDir := filepath.Join(testDir, "t", "configs")
	pkgCfgFile := filepath.Join(pkgDir, "snclient_append.ini")

	cfg := NewConfig(false)
	err := cfg.ParseINIFile(pkgCfgFile, nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/default")
	allowed, _ := section.GetString("allowed hosts")

	expected := "127.0.0.1, ::1, 192.168.0.1, 192.168.0.2,192.168.0.3"
	assert.Equalf(t, expected, allowed, "reading appended config")
}

func TestConfigLongLines(t *testing.T) {
	configText := `
[/settings/default]
allowed hosts  = 127.0.0.1, ::1, 192.168.1.1`

	for range 10000 {
		configText += ", 192.168.100.123"
	}
	configText += "\n"

	iniFile, _ := os.CreateTemp(t.TempDir(), "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	require.NoErrorf(t, err, "config written")
	cfg := NewConfig(false)
	err = cfg.ReadINI(iniFile.Name(), nil)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/default")
	allowed, _ := section.GetString("allowed hosts")

	assert.Containsf(t, allowed, "192.168.1.1", "reading appended config")
}

func TestConfigDefaults(t *testing.T) {
	configText := `
[/modules]
NodeExporterServer = enabled
`

	iniFile, _ := os.CreateTemp(t.TempDir(), "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	require.NoErrorf(t, err, "config written")

	snc := &Agent{}
	initSet, err := snc.ReadConfiguration([]string{iniFile.Name()})
	require.NoErrorf(t, err, "config parsed")

	section := initSet.config.Section("/settings/NodeExporter/server")
	port, _ := section.GetString("port")

	assert.Containsf(t, port, "8443", "reading default config")
}

func TestConfigHTTPInclude(t *testing.T) {
	snc := StartTestAgent(t, "")
	testPort := 55557

	// start mock http server
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", testPort),
		ReadTimeout:       DefaultSocketTimeout * time.Second,
		ReadHeaderTimeout: DefaultSocketTimeout * time.Second,
		WriteTimeout:      DefaultSocketTimeout * time.Second,
		IdleTimeout:       DefaultSocketTimeout * time.Second,
		ErrorLog:          NewStandardLog("WARN"),
	}
	httpConfig1 := `
[/settings/default]
allowed hosts  = 127.0.0.1, ::1, 192.168.1.1`
	httpConfig2 := `
[/settings/default]
allowed hosts  += 192.168.2.2`
	httpConfig3 := `
[/settings/default]
allowed hosts  += 192.168.3.3`

	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/plain")
		switch req.URL.Path {
		case "/tmp/test1.ini":
			res.WriteHeader(http.StatusOK)
			LogError2(res.Write([]byte(httpConfig1)))
		case "/tmp/test2.ini":
			res.WriteHeader(http.StatusOK)
			LogError2(res.Write([]byte(httpConfig2)))
		case "/tmp/test3.ini":
			if verifyRequestPassword(snc, req, "pass") {
				res.WriteHeader(http.StatusOK)
				LogError2(res.Write([]byte(httpConfig3)))
			} else {
				LogError2(res.Write([]byte("not allowed")))
				http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			}
		}
	})
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Debugf("mock http server finished with: %s", err.Error())
		}
	}()
	defer func() {
		LogError(server.Shutdown(context.TODO()))
	}()

	// wait up to 30 seconds for mock server to start
	testReq, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, fmt.Sprintf("http://localhost:%d", testPort), http.NoBody)
	require.NoErrorf(t, err, "request created")
	httpClient := snc.httpClient(&HTTPClientOptions{reqTimeout: DefaultSocketTimeout})
	for range 300 {
		res, err2 := httpClient.Do(testReq)
		if err2 != nil {
			time.Sleep(100 * time.Millisecond)

			continue
		}
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	configText := fmt.Sprintf(`
[/includes]
remote = http://localhost:%d/tmp/test1.ini

[/includes/another]
url = http://localhost:%d/tmp/test2.ini

[/includes]
auth = http://user:pass@localhost:%d/tmp/test3.ini
`, testPort, testPort, testPort)

	iniFile, _ := os.CreateTemp(t.TempDir(), "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err = iniFile.Close()
	require.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name(), snc)
	require.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/default")
	allowed, _ := section.GetString("allowed hosts")

	assert.Containsf(t, allowed, "192.168.1.1", "reading http config")
	assert.Containsf(t, allowed, "192.168.2.2", "reading http config")
	assert.Containsf(t, allowed, "192.168.3.3", "reading http config")

	StopTestAgent(t, snc)
}

func TestConfigQuotes(t *testing.T) {
	configText := `
[/settings/external scripts/alias]
alias_foo1=check_process process=postgres4.exe ok-syntax='Total processes = ${total}'
alias_foo2=check_service service="Eventlog" show-all "top-syntax=${list}" "detail-syntax=${name}:${state} " "crit=(state != 'running')"
`
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, "testfile.ini", nil)
	require.NoErrorf(t, err, "config parsed")
	snc := StartTestAgent(t, configText)
	StopTestAgent(t, snc)
}
