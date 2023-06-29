package snclient

import (
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
}
