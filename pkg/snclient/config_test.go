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
