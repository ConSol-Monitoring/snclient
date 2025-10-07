package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllRequiredMacros(t *testing.T) {
	check := &CheckData{
		name: "testcheck",
	}
	_, err := check.parseArgs([]string{
		"warn=test ne 10",
		"crit=blah in (5,1,3)",
		"ok = column1 eq 5",
		"detail-syntax = -%(column29)-",
	})
	require.NoError(t, err)

	macros := check.GetAllThresholdKeywords()
	assert.Equal(t, []string{"blah", "column1", "test"}, macros)

	macros = check.AllRequiredMacros()
	assert.Equal(t, []string{"blah", "column1", "column29", "test"}, macros)
}
