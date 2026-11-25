package snclient

import (
	"context"
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestCheckEventlog(t *testing.T) {
	snc := Agent{}
	// match nothing
	res := snc.RunCheck("check_eventlog", []string{"file=System", "filter=id < 0", "scan-range=-1h"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(
		t,
		`^OK - No entries found`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	// match nothing by id and source
	res = snc.RunCheck("check_eventlog", []string{
		"file=System",
		"filter=id in (65535) and source like 'snclient-no-such-source'",
		"scan-range=-1h",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(
		t,
		`^OK - No entries found`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
}

func TestCheckEventlogUnknownChannel(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_eventlog", []string{"file=snclient-no-such-channel"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "snclient-no-such-channel")
}

func TestCheckEventlogEventCodes(t *testing.T) {
	tests := []struct {
		test    string
		filters []string
		groups  [][]uint64
		errMsg  string
	}{
		{test: "no filters", filters: nil, groups: [][]uint64{}},
		{test: "single event code", filters: []string{"100"}, groups: [][]uint64{{100}}},
		{
			test:    "event codes with spaces",
			filters: []string{" 100 , 101 ,"},
			groups:  [][]uint64{{100, 101}},
		},
		{test: "event codes max value", filters: []string{"65535"}, groups: [][]uint64{{65535}}},
		{
			test:    "multiple event code filters",
			filters: []string{"100,101", "102"},
			groups:  [][]uint64{{100, 101}, {102}},
		},
		{
			test:    "event codes out of range",
			filters: []string{"65536"},
			errMsg:  "invalid event code \"65536\": must be an integer between 0 and 65535",
		},
		{
			test:    "negative event codes",
			filters: []string{"-1"},
			errMsg:  "invalid event code \"-1\"",
		},
		{
			test:    "event codes empty",
			filters: []string{" , "},
			errMsg:  "no event codes specified",
		},
	}

	for _, test := range tests {
		t.Run(test.test, func(t *testing.T) {
			check := &CheckEventlog{eventCodesFilters: test.filters}
			groups, err := check.parseEventCodes()
			if test.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errMsg)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.groups, groups)
		})
	}
}

func TestCheckEventlogMatchesEventCodes(t *testing.T) {
	assert.True(t, matchesEventCodes(100, [][]uint64{{100, 101}}))
	assert.False(t, matchesEventCodes(102, [][]uint64{{100, 101}}))
	assert.False(t, matchesEventCodes(102, [][]uint64{{100, 101}, {102}}))
	assert.True(t, matchesEventCodes(102, [][]uint64{{100, 101, 102}, {102}}))
	assert.False(t, matchesEventCodes(100, [][]uint64{{100, 101}, {102}}))
	assert.True(t, matchesEventCodes(100, nil))
}

func TestEventlogGetChannels(t *testing.T) {
	channels, err := getEventLogChannels(context.Background())
	require.NoError(t, err)
	assert.Contains(t, channels, "Application")
	assert.Contains(t, channels, "System")
}

func TestEventlogGetLog(t *testing.T) {
	events, err := getEventLog(context.Background(), "System", time.Now().Add(-7*24*time.Hour), nil)
	require.NoError(t, err)
	require.NotEmpty(t, events)

	for _, event := range events {
		assert.Equal(t, "System", event.LogFile)
		assert.NotEmpty(t, event.SourceName)
		assert.NotZero(t, event.TimeWritten)
		assert.NotZero(t, event.EventIdentifier)
	}
}

func TestEventlogGetLogUnknownChannel(t *testing.T) {
	_, err := getEventLog(context.Background(), "snclient-no-such-channel", time.Now().Add(-time.Hour), nil)
	require.Error(t, err)
}

func TestEventlogEventType(t *testing.T) {
	assert.Equal(t, "audit failure", eventLogType(0, keywordAuditFailure))
	assert.Equal(t, "audit success", eventLogType(0, keywordAuditSuccess))
	assert.Equal(t, "error", eventLogType(1, 0))
	assert.Equal(t, "error", eventLogType(2, 0))
	assert.Equal(t, "warning", eventLogType(3, 0))
	assert.Equal(t, "information", eventLogType(4, 0))
	assert.Equal(t, "verbose", eventLogType(5, 0))
	assert.Equal(t, "information", eventLogType(0, 0))
}

func TestEvtVariantSize(t *testing.T) {
	assert.Equal(t, uintptr(16), unsafe.Sizeof(evtVariant{}))
}

func TestEvtVariantUint64Value(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
		typ   uint32
		want  uint64
	}{
		{name: "byte", value: math.MaxUint8 | 1<<8, typ: evtVarTypeByte, want: math.MaxUint8},
		{name: "uint16", value: math.MaxUint16 | 1<<16, typ: evtVarTypeUInt16, want: math.MaxUint16},
		{name: "uint32", value: math.MaxUint32 | 1<<32, typ: evtVarTypeUInt32, want: math.MaxUint32},
		{name: "uint64", value: math.MaxUint64, typ: evtVarTypeUInt64, want: math.MaxUint64},
		{name: "hex int32", value: math.MaxUint32 | 1<<32, typ: evtVarTypeHexInt32, want: math.MaxUint32},
		{name: "hex int64", value: math.MaxUint64, typ: evtVarTypeHexInt64, want: math.MaxUint64},
		{name: "null", value: math.MaxUint64, typ: evtVarTypeNull, want: 0},
		{name: "wrong type", value: math.MaxUint64, typ: evtVarTypeString, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			variant := evtVariant{Value: test.value, Type: test.typ}
			assert.Equal(t, test.want, variant.uint64Value())
		})
	}
}

func TestEvtVariantStringValue(t *testing.T) {
	ptr := windows.StringToUTF16Ptr("snclient")
	variant := evtVariant{Type: evtVarTypeString}
	*(*unsafe.Pointer)(unsafe.Pointer(&variant.Value)) = unsafe.Pointer(ptr)

	assert.Equal(t, "snclient", variant.stringValue())
	assert.Empty(t, (&evtVariant{Type: evtVarTypeByte, Value: 1}).stringValue())
}

func TestEvtVariantTimeValue(t *testing.T) {
	want := time.Now().UTC().Truncate(time.Microsecond)
	filetime := windows.NsecToFiletime(want.UnixNano())
	variant := evtVariant{
		Value: uint64(filetime.HighDateTime)<<32 | uint64(filetime.LowDateTime),
		Type:  evtVarTypeFileTime,
	}

	assert.True(t, want.Equal(variant.timeValue()))
	assert.True(t, (&evtVariant{Type: evtVarTypeByte}).timeValue().IsZero())
}
