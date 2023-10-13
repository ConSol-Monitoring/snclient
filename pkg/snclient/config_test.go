package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigBasic(t *testing.T) {
	configText := `
[/test]
Key1 = Value1
Key2 = "Value2"
Key3 = 'Value3'
; comment
# also comment
	`
	cfg := NewConfig(true)
	err := cfg.ParseINI(strings.NewReader(configText), "testfile.ini")

	assert.NoErrorf(t, err, "config parsed")

	expData := ConfigData{
		"Key1": "Value1",
		"Key2": "Value2",
		"Key3": "Value3",
	}
	assert.Equalf(t, expData, cfg.Section("/test").data, "config parsed")
}

func TestConfigErrorI(t *testing.T) {
	configText := `
[/test]
Key1 = "Value1
	`
	cfg := NewConfig(true)
	err := cfg.ParseINI(strings.NewReader(configText), "testfile.ini")

	assert.Errorf(t, err, "config error found")
	assert.ErrorContains(t, err, "config error in testfile.ini:3: unclosed quotes")
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
	err := cfg.ParseINI(strings.NewReader(configText), "testfile.ini")
	assert.NoErrorf(t, err, "config parsed")

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
	iniFile, _ := os.CreateTemp("", "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	assert.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name())
	assert.NoErrorf(t, err, "config parsed")

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
	iniFile, _ := os.CreateTemp("", "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	assert.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name())
	assert.NoErrorf(t, err, "config parsed")

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
	iniFile, _ := os.CreateTemp("", "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err := iniFile.Close()
	assert.NoErrorf(t, err, "config written")
	cfg := NewConfig(true)
	err = cfg.ReadINI(iniFile.Name())
	assert.NoErrorf(t, err, "config parsed")

	section := cfg.Section("/settings/NRPE/server")
	nrpePort, _ := section.GetString("port")
	assert.Equalf(t, "12345", nrpePort, "got nrpe port")

	section = cfg.Section("/settings/WEB/server")
	webPort, _ := section.GetString("port")
	assert.Equalf(t, "1919", webPort, "got web port")
	webPassword, _ := section.GetString("password")
	assert.Equalf(t, "s00pers3cr3t", webPassword, "got web password")
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

; use ssl - security i important hmmkay
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
	err := cfg.ParseINI(strings.NewReader(configText), "test.ini")

	assert.NoErrorf(t, err, "parsed ini without error")
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

; use ssl - security i important hmmkay
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

	file, err := os.Open(pkgCfgFile)
	assert.NoErrorf(t, err, "open ini without error")

	data, err := os.ReadFile(pkgCfgFile)
	assert.NoErrorf(t, err, "read ini without error")
	origConfig := strings.TrimSpace(string(data))

	if runtime.GOOS == "windows" {
		// assume original config file has windows newlines
		origConfig = strings.ReplaceAll(origConfig, "\n", "\r\n")
	}

	cfg := NewConfig(false)
	err = cfg.ParseINI(file, pkgCfgFile)
	file.Close()

	assert.NoErrorf(t, err, "parse ini without error")
	assert.Equalf(t, origConfig, strings.TrimSpace(cfg.ToString()), "default config should not change when opened and saved unchanged")
}

func TestConfigRelativeIncludes(t *testing.T) {
	testDir, _ := os.Getwd()
	pkgDir := filepath.Join(testDir, "t", "configs")
	pkgCfgFile := filepath.Join(pkgDir, "snclient_incl.ini")

	file, err := os.Open(pkgCfgFile)
	assert.NoErrorf(t, err, "open ini without error")

	cfg := NewConfig(true)
	err = cfg.ParseINI(file, pkgCfgFile)
	file.Close()
	assert.NoErrorf(t, err, "config parsed")

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
