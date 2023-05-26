package humanize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBytes(t *testing.T) {
	tests := []struct {
		in  string
		res uint64
		err bool
	}{
		{"1B", 1, false},
		{"1MB", 1000000, false},
		{"1MiB", 1048576, false},
		{"1.5GB", 1500000000, false},
		{"1.5GiB", 1610612736, false},
		{"xyz", 0, true},
	}

	for _, tst := range tests {
		res, err := ParseBytes(tst.in)
		if tst.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
		assert.Equalf(t, tst.res, res, "ParseBytes: %s -> %d", tst.in, res)
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
