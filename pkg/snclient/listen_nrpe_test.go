package snclient

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/consol-monitoring/snclient/pkg/nrpe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerNRPE(t *testing.T) {
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
port = 0
allow arguments = false
use ssl = false
`

	snc := StartTestAgent(t, config)
	testPort := testListenerPort(t, snc)

	con, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", testPort), 10*time.Second)
	require.NoErrorf(t, err, "connection established")

	req := nrpe.BuildPacketV4(nrpe.NrpeQueryPacket, 0, []byte("check_snclient_version"))
	err = req.Write(con)
	require.NoErrorf(t, err, "request send")

	res, err := nrpe.ReadNrpePacket(con)
	require.NoErrorf(t, err, "response read")
	cmd, _ := res.Data()
	assert.Regexpf(t, "^SNClient", cmd, "response matches")

	StopTestAgent(t, snc)
}
