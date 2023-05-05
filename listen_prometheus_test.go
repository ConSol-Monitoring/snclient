package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerPrometheus(t *testing.T) {
	assert.Implements(t, (*RequestHandlerHTTP)(nil), new(HandlerPrometheus))
}
