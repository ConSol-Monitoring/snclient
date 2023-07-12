package snclient

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	cfg := NewConfig()
	err := cfg.parseINI(strings.NewReader(configText), "testfile.ini")

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
	cfg := NewConfig()
	err := cfg.parseINI(strings.NewReader(configText), "testfile.ini")

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
	cfg := NewConfig()
	err := cfg.parseINI(strings.NewReader(configText), "testfile.ini")
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
	iniFile, err := ioutil.TempFile("", "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err = iniFile.Close()
	assert.NoErrorf(t, err, "config written")
	cfg := NewConfig()
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
custom_ini = %s

	`, configsDir, customDir)
	iniFile, err := ioutil.TempFile("", "snclient-*.ini")
	defer os.Remove(iniFile.Name())
	_, _ = iniFile.WriteString(configText)
	err = iniFile.Close()
	assert.NoErrorf(t, err, "config written")
	cfg := NewConfig()
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
