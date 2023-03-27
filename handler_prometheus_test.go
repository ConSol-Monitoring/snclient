package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerPrometheus(t *testing.T) {
	t.Parallel()
	assert.Implements(t, (*RequestHandlerHTTP)(nil), new(HandlerPrometheus))
}
