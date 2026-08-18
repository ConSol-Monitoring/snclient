package humanize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBytes(t *testing.T) {
	tests := []struct {
		in  string
		res uint64
		err bool
	}{
		{"1B", 1, false},
		{"1MB", 1000000, false},
		{"1M", 1000000, false},
		{"1MiB", 1048576, false},
		{"1.5GB", 1500000000, false},
		{"1.5G", 1500000000, false},
		{"1.5GiB", 1610612736, false},
		{"xyz", 0, true},
	}

	for _, tst := range tests {
		res, err := ParseBytes(tst.in)
		if tst.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equalf(t, tst.res, res, "ParseBytes: %s -> %d", tst.in, res)
	}
}

func TestParseBytesPerSec(t *testing.T) {
	tests := []struct {
		in  string
		res float64
		err bool
	}{
		{"1 B/s", 1, false},
		{"1B/s", 1, false},
		{"1 KiB/s", 1024, false},
		{"1 KiB /s", 1024, false},
		{"1 MiB/s", 1048576, false},
		{"1.5 MiB/s", 1.5 * 1048576, false},
		{"1 MB/s", 1000000, false},
		{"1 GB/s", 1000000000, false},
		{"80GB", 80000000000, false},
		{"80 GB", 80000000000, false},
		{"4194304", 4194304, false},
		{"12345.67", 12345.67, false},
		{"1 Mbps", 125000, false},
		{"100 Mbps", 12500000, false},
		{"1 Gbps", 125000000, false},
		{"1 kb/s", 1000, false},
		{"", 0, true},
		{"xyz", 0, true},
		{"1 xyz/s", 0, true},
	}

	for _, tst := range tests {
		res, err := ParseBytesPerSec(tst.in)
		if tst.err {
			require.Errorf(t, err, "ParseBytesPerSec: %s should error", tst.in)
		} else {
			require.NoErrorf(t, err, "ParseBytesPerSec: %s", tst.in)
		}
		assert.InDeltaf(t, tst.res, res, 0.0001, "ParseBytesPerSec: %s -> %f", tst.in, res)
	}
}

func TestBytes(t *testing.T) {
	tests := []struct {
		in  uint64
		res string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{800000, "800 kB"},
		{1000000, "1 MB"},
		{1200000, "1200 kB"},
		{1500000, "1500 kB"},
		{2000000, "2 MB"},
		{2990000, "2990 kB"},
		{5499999, "5 MB"},
		{5500000, "6 MB"},
	}

	for _, tst := range tests {
		res := Bytes(tst.in)
		assert.Equalf(t, tst.res, res, "Bytes: %d -> %s", tst.in, res)
	}
}

func TestIBytes(t *testing.T) {
	tests := []struct {
		in  uint64
		res string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{819200, "800 KiB"},
		{1048576, "1 MiB"},
		{1228800, "1200 KiB"},
		{1536000, "1500 KiB"},
		{2097152, "2 MiB"},
		{3061760, "2990 KiB"},
		{5767167, "5 MiB"},
		{5767168, "6 MiB"},
	}

	for _, tst := range tests {
		res := IBytes(tst.in)
		assert.Equalf(t, tst.res, res, "Bytes: %d -> %s", tst.in, res)
	}
}

func TestIBytesF(t *testing.T) {
	tests := []struct {
		in  uint64
		pre int
		res string
	}{
		{0, 3, "0 B"},
		{1, 3, "1 B"},
		{819200, 3, "800.000 KiB"},
		{1048576, 3, "1.000 MiB"},
		{1228800, 3, "1.172 MiB"},
		{1536000, 3, "1.465 MiB"},
		{2097152, 3, "2.000 MiB"},
		{3061760, 3, "2.920 MiB"},
		{5767167, 3, "5.500 MiB"},
		{5767168, 3, "5.500 MiB"},
	}

	for _, tst := range tests {
		res := IBytesF(tst.in, tst.pre)
		assert.Equalf(t, tst.res, res, "Bytes: %d -> %s", tst.in, res)
	}
}
