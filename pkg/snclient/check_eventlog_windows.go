//go:build windows

package snclient

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"golang.org/x/sys/windows"
)

//nolint:funlen // the check setup and the event loop are easier to read together
func (l *CheckEventlog) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	eventCodes, err := l.parseEventCodes()
	if err != nil {
		return nil, err
	}

	if len(l.files) == 0 {
		channels, err2 := getEventLogChannels(ctx)
		if err2 != nil {
			return nil, fmt.Errorf("failed to enumerate event log channels: %w", err2)
		}
		l.files = append(l.files, channels...)
	}

	lookBack, err := utils.ExpandDuration(l.scanRange)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse scan-range: %w", err)
	}
	if lookBack < 0 {
		lookBack *= -1
	}
	scanLookBack := time.Now().Add(-time.Second * time.Duration(lookBack))

	uniqueIndexList := map[string]map[string]string{}
	filterUnique := false

	switch l.uniqueIndex {
	case "", "0", "false", "no":
		l.uniqueIndex = ""
	case "1":
		filterUnique = true
		l.uniqueIndex = DefaultUniqueIndex
	default:
		filterUnique = true
	}

	queried := 0
	var firstErr error

	// Rendering event messages from raw event data is expensive. Build simple
	// matchers for the conditions that can be checked before the message is rendered.
	needsMessage := filterNeedsMessage(check.filter)
	matchers := make([]func(*eventLogEvent) bool, 0, 2)
	if len(eventCodes) > 0 {
		matchers = append(matchers, func(event *eventLogEvent) bool {
			return matchesEventCodes(event.EventCode, eventCodes)
		})
	}
	if !needsMessage && len(check.filter) > 0 {
		matchers = append(matchers, func(event *eventLogEvent) bool {
			return check.MatchMapCondition(check.filter, l.eventListData(event, ""), false)
		})
	}

	for _, file := range l.files {
		log.Tracef("fetching eventlog: %s", file)
		fileEvent, err := getEventLog(ctx, file, scanLookBack, matchers)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Tracef("skipping event log %q: %s", file, err)

			continue
		}
		queried++

		for i := range fileEvent {
			event := &fileEvent[i]
			listData := l.eventListData(event, event.Message)
			if !filterUnique {
				check.listData = append(check.listData, listData)

				continue
			}

			// filter out duplicate events based on the unique-index argument
			uniqueID := ReplaceMacros(l.uniqueIndex, check.timezone, listData)
			log.Tracef("expanded unique filter: %s", uniqueID)
			if prevEntry, ok := uniqueIndexList[uniqueID]; ok {
				count := convert.Int64(prevEntry["_count"])
				prevEntry["_count"] = fmt.Sprintf("%d", count+1)
			} else {
				check.listData = append(check.listData, listData)
				listData["_count"] = "1"
				uniqueIndexList[uniqueID] = listData
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading event logs: %w", err)
	}

	if queried == 0 && firstErr != nil {
		return nil, firstErr
	}

	return check.Finalize()
}

func (l *CheckEventlog) eventListData(event *eventLogEvent, message string) map[string]string {
	if l.truncateMessage > 0 && len(message) > l.truncateMessage {
		message = message[:l.truncateMessage]
	}

	return map[string]string{
		"channel":         event.Channel,
		"provider":        event.Provider,
		"file":            event.Channel,
		"log":             event.Channel,
		"source":          event.Provider,
		"computer":        event.ComputerName,
		"id":              fmt.Sprintf("%d", event.EventCode),
		"eventidentifier": fmt.Sprintf("%d", event.EventIdentifier),
		"level":           event.Type,
		"message":         message,
		"written":         fmt.Sprintf("%d", event.TimeWritten.Unix()),
	}
}

func filterNeedsMessage(filter ConditionList) bool {
	keywords, err := filter.GetListOfKeywords()
	if err != nil {
		return true
	}

	return slices.Contains(keywords, "message")
}

// CheckEventlogMaxEventCode is the highest value of the 16 bit EventID.
const CheckEventlogMaxEventCode = 65535

func (l *CheckEventlog) parseEventCodes() ([][]uint64, error) {
	groups := make([][]uint64, 0, len(l.eventCodesFilters))
	for _, filter := range l.eventCodesFilters {
		values := l.splitFilterValues(filter)
		if len(values) == 0 {
			return nil, fmt.Errorf("event-codes: no event codes specified")
		}

		group := make([]uint64, 0, len(values))
		for _, value := range values {
			eventCode, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("event-codes: invalid event code %q: must be an integer between 0 and %d",
					value, CheckEventlogMaxEventCode)
			}
			group = append(group, eventCode)
		}
		groups = append(groups, group)
	}

	return groups, nil
}

func matchesEventCodes(eventCode uint64, groups [][]uint64) bool {
	for _, group := range groups {
		if !slices.Contains(group, eventCode) {
			return false
		}
	}

	return true
}

func (l *CheckEventlog) splitFilterValues(filter string) []string {
	values := []string{}
	for value := range strings.SplitSeq(filter, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		values = append(values, value)
	}

	return values
}

// eventLogEvent is a single entry read from a Windows event log channel.
type eventLogEvent struct {
	ComputerName    string
	Channel         string
	Message         string
	Provider        string
	Type            string
	TimeWritten     time.Time
	EventCode       uint64
	EventIdentifier uint64
}

// Constants are taken from winevt.h unless noted otherwise.
const (
	evtQueryChannelPath      = 0x1
	evtQueryForwardDirection = 0x100

	evtRenderEventValues = 0

	evtFormatMessageEvent = 1

	evtRenderContextValues = 0

	evtVarTypeNull     = 0
	evtVarTypeString   = 1
	evtVarTypeByte     = 4
	evtVarTypeUInt16   = 6
	evtVarTypeUInt32   = 8
	evtVarTypeUInt64   = 10
	evtVarTypeFileTime = 17
	evtVarTypeHexInt32 = 20
	evtVarTypeHexInt64 = 21
	evtVarTypeArray    = 128

	// WINEVENT_KEYWORD_AUDIT_* from winmeta.h.
	keywordAuditFailure = 0x10000000000000
	keywordAuditSuccess = 0x20000000000000
)

// Local constants for the value path indexes, buffer sizes and masks.
const (
	// indexes into eventLogValuePaths
	valueProvider    = 0
	valueEventID     = 1
	valueLevel       = 2
	valueTimeCreated = 3
	valueChannel     = 4
	valueComputer    = 5
	valueKeywords    = 6
	valuePathCount   = 7

	channelPathBufferSize = 512
	eventBatchSize        = 64
	uint64Size            = 8

	maskByte   = 0xFF
	maskUInt16 = 0xFFFF
	maskUInt32 = 0xFFFFFFFF
)

// eventLogValuePaths are core System properties from the Event schema.
// They exist on client and server Windows.
// A property that is missing from an event comes back as EvtVarTypeNull, not as a shorter result array.
// Their order matches the value* constants above.
var eventLogValuePaths = [valuePathCount]string{
	"Event/System/Provider/@Name",
	"Event/System/EventID",
	"Event/System/Level",
	"Event/System/TimeCreated/@SystemTime",
	"Event/System/Channel",
	"Event/System/Computer",
	"Event/System/Keywords",
}

var (
	wevtapiDll = windows.NewLazySystemDLL("wevtapi.dll")

	procEvtOpenChannelEnum       = wevtapiDll.NewProc("EvtOpenChannelEnum")
	procEvtNextChannelPath       = wevtapiDll.NewProc("EvtNextChannelPath")
	procEvtQuery                 = wevtapiDll.NewProc("EvtQuery")
	procEvtNext                  = wevtapiDll.NewProc("EvtNext")
	procEvtCreateRenderContext   = wevtapiDll.NewProc("EvtCreateRenderContext")
	procEvtRender                = wevtapiDll.NewProc("EvtRender")
	procEvtOpenPublisherMetadata = wevtapiDll.NewProc("EvtOpenPublisherMetadata")
	procEvtFormatMessage         = wevtapiDll.NewProc("EvtFormatMessage")
	procEvtClose                 = wevtapiDll.NewProc("EvtClose")
)

// getEventLogChannels returns all event log channels registered on the system.
func getEventLogChannels(ctx context.Context) ([]string, error) {
	channelEnum, _, err := procEvtOpenChannelEnum.Call(0, 0)
	if channelEnum == 0 {
		return nil, fmt.Errorf("EvtOpenChannelEnum: %w", err)
	}
	channelEnumHandle := windows.Handle(channelEnum)
	defer evtClose(channelEnumHandle)

	channels := []string{}
	buf := make([]uint16, channelPathBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("enumerating event log channels: %w", err)
		}

		var used uint32
		ret, _, err := procEvtNextChannelPath.Call(
			uintptr(channelEnumHandle),
			uintptr(len(buf)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&used)),
		)
		if ret != 0 {
			channels = append(channels, windows.UTF16ToString(buf[:used]))

			continue
		}
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			return channels, nil
		}
		if !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) {
			return nil, fmt.Errorf("EvtNextChannelPath: %w", err)
		}
		if used == 0 {
			return nil, fmt.Errorf("EvtNextChannelPath: %w", err)
		}
		buf = make([]uint16, used)
	}
}

// getEventLog reads all events newer than newerThan from a single event log channel.
// If matchers is not empty, only events matching every matcher are returned and their message is rendered.
// If matchers is empty, all messages are rendered.
//
//nolint:funlen // the query setup is inlined into the event loop
func getEventLog(ctx context.Context, channel string, newerThan time.Time, matchers []func(*eventLogEvent) bool) ([]eventLogEvent, error) {
	xpath := fmt.Sprintf(`*[System[TimeCreated[timediff(@SystemTime) <= %d]]]`, time.Since(newerThan).Milliseconds())

	channelPtr, err := syscall.UTF16PtrFromString(channel)
	if err != nil {
		return nil, fmt.Errorf("converting channel name: %w", err)
	}
	queryPtr, err := syscall.UTF16PtrFromString(xpath)
	if err != nil {
		return nil, fmt.Errorf("converting query: %w", err)
	}

	// Oldest first, like the previous WMI backend, using the evtQueryForwardDirection flag
	resultSetHandle, _, err := procEvtQuery.Call(
		0,
		uintptr(unsafe.Pointer(channelPtr)),
		uintptr(unsafe.Pointer(queryPtr)),
		uintptr(evtQueryChannelPath|evtQueryForwardDirection),
	)
	if resultSetHandle == 0 {
		return nil, fmt.Errorf("EvtQuery(%q): %w", channel, err)
	}
	resultSet := windows.Handle(resultSetHandle)
	defer evtClose(resultSet)

	paths := make([]*uint16, len(eventLogValuePaths))
	for i, valuePath := range eventLogValuePaths {
		paths[i] = windows.StringToUTF16Ptr(valuePath)
	}
	renderContextHandle, _, err := procEvtCreateRenderContext.Call(
		uintptr(len(paths)),
		uintptr(unsafe.Pointer(&paths[0])),
		evtRenderContextValues,
	)
	if renderContextHandle == 0 {
		return nil, fmt.Errorf("EvtCreateRenderContext: %w", err)
	}
	renderContext := windows.Handle(renderContextHandle)
	defer evtClose(renderContext)

	publishers := &eventLogPublisherCache{handles: map[string]windows.Handle{}}
	defer func() {
		for _, handle := range publishers.handles {
			evtClose(handle)
		}
	}()

	events := []eventLogEvent{}
	batch := make([]windows.Handle, eventBatchSize)
	for {
		if err := ctx.Err(); err != nil {
			return events, fmt.Errorf("reading event log %q: %w", channel, err)
		}

		var returnedEventCount uint32
		ret, _, err := procEvtNext.Call(
			uintptr(resultSet),
			uintptr(len(batch)),
			uintptr(unsafe.Pointer(&batch[0])),
			0,
			0,
			uintptr(unsafe.Pointer(&returnedEventCount)),
		)
		// Some Windows versions report an exhausted query as ERROR_INVALID_OPERATION with no handles.
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) ||
			(errors.Is(err, windows.ERROR_INVALID_OPERATION) && returnedEventCount == 0) {
			return events, nil
		}
		if ret == 0 {
			return nil, fmt.Errorf("EvtNext(%q): %w", channel, err)
		}

		for _, eventHandle := range batch[:returnedEventCount] {
			event, err := renderEventLogEvent(renderContext, eventHandle, channel)
			if err != nil {
				evtClose(eventHandle)
				log.Debugf("skipping event from %q: %s", channel, err)

				continue
			}
			if !matchesAll(matchers, &event) {
				evtClose(eventHandle)

				continue
			}
			if event.Provider != "" {
				event.Message = publishers.format(eventHandle, event.Provider)
			}
			evtClose(eventHandle)
			events = append(events, event)
		}
	}
}

func matchesAll(matchers []func(*eventLogEvent) bool, event *eventLogEvent) bool {
	for _, matcher := range matchers {
		if !matcher(event) {
			return false
		}
	}

	return true
}

func renderEventLogEvent(renderContext, eventHandle windows.Handle, channel string) (eventLogEvent, error) {
	var used, count uint32

	ret, _, err := procEvtRender.Call(
		uintptr(renderContext),
		uintptr(eventHandle),
		evtRenderEventValues,
		0,
		0,
		uintptr(unsafe.Pointer(&used)),
		uintptr(unsafe.Pointer(&count)),
	)
	if ret == 0 && !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) {
		return eventLogEvent{}, fmt.Errorf("EvtRender: %w", err)
	}

	var values []evtVariant
	if ret == 0 && used > 0 {
		buf := make([]uint64, (used+uint64Size-1)/uint64Size)
		ret, _, err = procEvtRender.Call(
			uintptr(renderContext),
			uintptr(eventHandle),
			evtRenderEventValues,
			uintptr(used),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&used)),
			uintptr(unsafe.Pointer(&count)),
		)
		if ret == 0 {
			return eventLogEvent{}, fmt.Errorf("EvtRender: %w", err)
		}
		values = unsafe.Slice((*evtVariant)(unsafe.Pointer(&buf[0])), int(count))
	}

	if len(values) < valuePathCount {
		return eventLogEvent{}, fmt.Errorf("expected %d values, got %d", valuePathCount, len(values))
	}

	provider := values[valueProvider].stringValue()
	eventID := values[valueEventID].uint64Value()

	event := eventLogEvent{
		ComputerName:    values[valueComputer].stringValue(),
		Channel:         values[valueChannel].stringValue(),
		Provider:        provider,
		Type:            eventLogType(values[valueLevel].uint64Value(), values[valueKeywords].uint64Value()),
		TimeWritten:     values[valueTimeCreated].timeValue(),
		EventCode:       eventID & maskUInt16,
		EventIdentifier: eventID,
	}
	if event.Channel == "" {
		event.Channel = channel
	}

	return event, nil
}

func eventLogType(level, keywords uint64) string {
	switch {
	case keywords&keywordAuditFailure != 0:
		return "audit failure"
	case keywords&keywordAuditSuccess != 0:
		return "audit success"
	}

	switch level {
	case 1, 2:
		// WMI reports level 1 (critical) as "Error" as well, keep this mapping
		// so existing level filters keep working.
		return "error"
	case 3:
		return "warning"
	case 4:
		return "information"
	case 5:
		return "verbose"
	default:
		return "information"
	}
}

// eventLogPublisherCache keeps one publisher metadata handle per provider for the duration of a query.
// Formatting the event message needs this handle.
// Handles are lazily opened and cached using the map
type eventLogPublisherCache struct {
	handles map[string]windows.Handle
}

func (c *eventLogPublisherCache) format(event windows.Handle, provider string) string {
	handle, ok := c.handles[provider]
	if !ok {
		publisherPtr, err := syscall.UTF16PtrFromString(provider)
		if err == nil {
			ret, _, _ := procEvtOpenPublisherMetadata.Call(
				0,
				uintptr(unsafe.Pointer(publisherPtr)),
				0,
				0,
				0,
			)
			handle = windows.Handle(ret)
		}
		c.handles[provider] = handle
	}
	if handle == 0 {
		return ""
	}

	var used uint32
	ret, _, err := procEvtFormatMessage.Call(
		uintptr(handle),
		uintptr(event),
		0,
		0,
		0,
		evtFormatMessageEvent,
		0,
		0,
		uintptr(unsafe.Pointer(&used)),
	)
	if ret != 0 || !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) || used == 0 {
		return ""
	}

	buf := make([]uint16, used)
	ret, _, _ = procEvtFormatMessage.Call(
		uintptr(handle),
		uintptr(event),
		0,
		0,
		0,
		evtFormatMessageEvent,
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&used)),
	)
	if ret == 0 {
		return ""
	}

	return windows.UTF16ToString(buf[:used])
}

func evtClose(handle windows.Handle) {
	if handle != 0 {
		_, _, _ = procEvtClose.Call(uintptr(handle))
	}
}

// evtVariant mirrors the EVT_VARIANT structure. The union is stored in Value.
// It is 16 bytes on both 32 and 64 bit Windows.
type evtVariant struct {
	Value uint64
	_     uint32 // count, not used
	Type  uint32
}

func (v *evtVariant) variantType() uint32 {
	return v.Type &^ evtVarTypeArray
}

func (v *evtVariant) stringValue() string {
	if v.variantType() != evtVarTypeString {
		return ""
	}

	ptr := *(*unsafe.Pointer)(unsafe.Pointer(&v.Value))

	return windows.UTF16PtrToString((*uint16)(ptr))
}

func (v *evtVariant) uint64Value() uint64 {
	switch v.variantType() {
	case evtVarTypeByte:
		return v.Value & maskByte
	case evtVarTypeUInt16:
		return v.Value & maskUInt16
	case evtVarTypeUInt32, evtVarTypeHexInt32:
		return v.Value & maskUInt32
	case evtVarTypeUInt64, evtVarTypeHexInt64:
		return v.Value
	default:
		return 0
	}
}

func (v *evtVariant) timeValue() time.Time {
	switch v.variantType() {
	case evtVarTypeFileTime:
		filetime := *(*windows.Filetime)(unsafe.Pointer(&v.Value))

		return time.Unix(0, filetime.Nanoseconds())
	case evtVarTypeString:
		parsed, _ := time.Parse(time.RFC3339Nano, v.stringValue())

		return parsed
	default:
		return time.Time{}
	}
}
