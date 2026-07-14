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

func TestCheckingFilterKeywordsOnCheckAttributes(t *testing.T) {
	check := &CheckData{
		name:        "testcheck",
		description: "This test check simply has couple arguments.",
		attributes: []CheckAttribute{
			{
				name:        "attribute1",
				description: "attribute1",
			},
			{
				name:        "attribute2",
				description: "attribute2",
			},
			{
				name:        "attribute3",
				description: "attribute3",
			},
		},
	}

	_, err := check.parseArgs([]string{"filter='(attribute1 eq value1) and (attribute2 like value2) and (attribute3 != value3)'", "filter='attribute4 eq value4'"})
	require.NoError(t, err, "should not have a problem parsing arguments")

	err = check.checkFilterKeywordsAgainstAttributeNames()
	require.Error(t, err, "should error due to filter using keyword 'attribute4', which is not an attribute of check")
}
