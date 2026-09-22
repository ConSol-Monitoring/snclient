//go:build windows || linux

package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckServiceExcludeWildcard(t *testing.T) {
	assert.True(t, matchesServiceExclude([]string{"Wildcard*"}, "Wildcard"))
	assert.True(t, matchesServiceExclude([]string{"Wildcard*"}, "WildcardTest"))
}
