package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPerfConfig(t *testing.T) {
	perf, err := NewPerfConfig("used(unit:G;suffix:'s'; prefix:'pre') used %(ignored:true) *(unit:GiB)  ")
	assert.NoErrorf(t, err, "no error in NewPerfConfig")

	exp := []PerfConfig{
		{Selector: "used", Unit: "G", Suffix: "s", Prefix: "pre"},
		{Selector: "used %", Ignore: true},
		{Selector: "*", Unit: "GiB"},
	}
	assert.Equalf(t, exp, perf, "NewPerfConfig parsed correctly")
}
