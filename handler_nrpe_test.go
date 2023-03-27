package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerNRPE(t *testing.T) {
	t.Parallel()
	assert.Implements(t, (*RequestHandlerTCP)(nil), new(HandlerNRPE))
}
