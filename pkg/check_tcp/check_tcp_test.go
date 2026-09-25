package check_tcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		hostname string
		ipv4     bool
		ipv6     bool
		port     int
		wantErr  bool
	}{
		{name: "alias form -4 -p 22 host", args: []string{"-4", "-p", "22", "localhost"}, hostname: "localhost", ipv4: true, port: 22},
		{name: "-6 with positional host", args: []string{"-6", "example.com"}, hostname: "example.com", ipv6: true},
		{name: "positional host only", args: []string{"localhost"}, hostname: "localhost"},
		{name: "-H host with -4", args: []string{"-H", "h", "-4"}, hostname: "h", ipv4: true},
		{name: "-4 and -6 together", args: []string{"-H", "h", "-4", "-6"}, wantErr: true},
		{name: "unknown extra argument", args: []string{"-H", "h", "-4", "extra"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			opts, err := parseArgs(test.args)
			if test.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.hostname, opts.Hostname)
			assert.Equal(t, test.ipv4, opts.IPv4)
			assert.Equal(t, test.ipv6, opts.IPv6)
			assert.Equal(t, test.port, opts.Port)
		})
	}
}

func TestDialHostLiteralIP(t *testing.T) {
	ctx := context.Background()

	opts := &tcpOpts{Hostname: "1.2.3.4"}
	host, err := opts.dialHost(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", host)

	opts = &tcpOpts{Hostname: "1.2.3.4", IPv4: true}
	host, err = opts.dialHost(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", host)

	opts = &tcpOpts{Hostname: "1.2.3.4", IPv6: true}
	_, err = opts.dialHost(ctx)
	require.ErrorContains(t, err, "no IPv6 address found")

	opts = &tcpOpts{Hostname: "2001:db8::1", IPv6: true}
	host, err = opts.dialHost(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2001:db8::1", host)
}
