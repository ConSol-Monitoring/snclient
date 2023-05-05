package snclient

import (
	"net"
	"regexp"
	"testing"
	"time"

	"pkg/nrpe"

	"github.com/stretchr/testify/assert"
)

func TestHandlerNRPE(t *testing.T) {
	t.Parallel()
	assert.Implements(t, (*RequestHandlerTCP)(nil), new(HandlerNRPE))
}

func TestNRPE(t *testing.T) {
	t.Parallel()
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
	snc := StartTestAgent(t, config, []string{})

	con, err := net.DialTimeout("tcp", "127.0.0.1:45666", 10*time.Second)
	assert.NoErrorf(t, err, "connection established")

	req := nrpe.BuildPacketV4(nrpe.NrpeQueryPacket, 0, []byte("check_snclient_version"))
	err = req.Write(con)
	assert.NoErrorf(t, err, "request send")

	res, err := nrpe.ReadNrpePacket(con)
	assert.NoErrorf(t, err, "response read")
	cmd, _ := res.Data()
	assert.Regexpf(t, regexp.MustCompile("^SNClient"), cmd, "response matches")

	StopTestAgent(t, snc)
}
