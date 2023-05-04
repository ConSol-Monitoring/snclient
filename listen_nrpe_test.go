package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerNRPE(t *testing.T) {
	t.Parallel()
	assert.Implements(t, (*RequestHandlerTCP)(nil), new(HandlerNRPE))
}

func TestNRPE(t *testing.T) {
	config := `
[/modules]
NRPEServer = enabled

[/settings/log]
file name = stderr
level = error

[/settings/NRPE/server]
allow nasty characters = false
port = 45666
allow arguments = false
use ssl = false
`
	err := StartTestAgent(t, config, []string{})
	assert.NoErrorf(t, err, "test agent started")

	StopTestAgent(t)
}
