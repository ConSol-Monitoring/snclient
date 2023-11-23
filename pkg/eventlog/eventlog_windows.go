package eventlog

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/elastic/beats/v7/libbeat/logp"
	evsys "github.com/elastic/beats/v7/winlogbeat/sys/winevent"
	"github.com/elastic/beats/v7/winlogbeat/sys/wineventlog"
	"github.com/kdar/factorlog"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

type EventLog struct {
	LogChannel string
	TimeDiff   uint64
	log        *factorlog.FactorLog

	bookmark *evsys.Event
}

// <Select Path="Application">*[System[TimeCreated[timediff(@SystemTime) &lt;= 3600000]]]</Select>
type xmlEventQuerySelect struct {
	Channel string `xml:"Path,attr"`
	Query   string `xml:",chardata"`
}

type xmlEventQuery struct {
	ID      int                   `xml:"Id,attr"`
	Channel string                `xml:"Path,attr"`
	Select  []xmlEventQuerySelect `xml:"Select"`
}

type xmlEventList struct {
	XMLName xml.Name      `xml:"QueryList"`
	Query   xmlEventQuery `xml:"Query"`
}

func NewEventLog(logchan string, log *factorlog.FactorLog) *EventLog {
	return &EventLog{
		LogChannel: logchan,
		log:        log,
	}
}

func queryStringFromChannels(channels []string, timeDiffSeconds uint64) (string, error) {
	if len(channels) < 1 {
		return "", fmt.Errorf("missing channel for event log query")
	}

	selects := make([]xmlEventQuerySelect, 0)
	for _, channel := range channels {
		query := "*"
		if timeDiffSeconds > 0 {
			query += fmt.Sprint("[System[TimeCreated[timediff(@SystemTime) <= ", timeDiffSeconds*1000, "]]]")
		}
		selects = append(selects, xmlEventQuerySelect{
			Channel: channel,
			Query:   query,
		})
	}

	data, err := xml.Marshal(&xmlEventList{
		Query: xmlEventQuery{
			ID:      0,
			Channel: channels[0],
			Select:  selects,
		},
	})
	if err != nil {
		return "", fmt.Errorf("json error: %s", err.Error())
	}

	return string(data), nil
}

func (el *EventLog) prepare() (string, wineventlog.EvtHandle, wineventlog.EvtSubscribeFlag, error) {
	var (
		err      error
		bookmark wineventlog.EvtHandle
		flags    = wineventlog.EvtSubscribeStartAtOldestRecord
	)
	el.log.Debugf("Event Log: prepare query")

	var timeDiff uint64 = 3600
	if el.TimeDiff > 0 {
		timeDiff = el.TimeDiff
	}

	if el.bookmark != nil {
		timeDiff = 0
		bookmark, err = wineventlog.CreateBookmarkFromRecordID(el.bookmark.Channel, el.bookmark.RecordID)
		if err != nil {
			el.log.Errorf("could not create bookmark: %s", err.Error())
			bookmark = 0
		} else {
			flags = wineventlog.EvtSubscribeStartAfterBookmark
		}
	}

	query, err := queryStringFromChannels([]string{el.LogChannel}, timeDiff)
	if err != nil {
		if bookmark != 0 {
			_ = bookmark.Close()
		}

		return "", 0, 0, errors.Wrap(err, "could not create query xml")
	}

	return query, bookmark, flags, nil
}

func (el *EventLog) Query() ([]*evsys.Event, error) {
	query, bookmark, flags, err := el.prepare()
	if err != nil {
		return nil, err
	}
	if bookmark != 0 {
		defer bookmark.Close()
	}

	el.log.Debugf("Event Log: create windows event handle")
	signalHandle, err := windows.CreateEvent(nil, 1, 1, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not create windows event handle")
	}
	defer func() {
		err := windows.CloseHandle(signalHandle)
		if err != nil {
			el.log.Errorf("Event Log: close failed: %s", err.Error())
		}
	}()

	el.log.Debugf("Event Log: subscribe to windows event log with query: %s", query)
	subscription, err := wineventlog.Subscribe(0, signalHandle, "", query, bookmark, flags)
	if err != nil {
		return nil, errors.Wrap(err, "could not subscribe to event log")
	}
	defer subscription.Close()

	return (el.getEventFromSubscription(subscription))
}

func (el *EventLog) getEventFromSubscription(subscription wineventlog.EvtHandle) ([]*evsys.Event, error) {
	el.log.Debugf("Event Log: fetch event log handles")
	iter, err := wineventlog.NewEventIterator(wineventlog.WithSubscription(subscription))
	if err != nil {
		return nil, errors.Wrap(err, "could not create event iterator from subscription")
	}
	defer iter.Close()

	events := make([]*evsys.Event, 0)

	logger := logp.L()
	renderer, err := wineventlog.NewRenderer(0, logger)
	if err != nil {
		return nil, fmt.Errorf("wineventlog error: %s", err.Error())
	}
	defer renderer.Close()

	for {
		eventHandle, more := iter.Next()
		if !more {
			break
		}

		event, err := renderer.Render(eventHandle)
		if err != nil {
			el.log.Debugf("Event Log: could not render event: %s", err.Error())
		}
		if event != nil {
			// user generated message contains: "[{{eventParam $ 1}}]" maybe there is a renderer but for now
			// just workaround by joining all eventdata values
			if strings.HasPrefix(event.Message, "[{{eventParam") || event.Message == "" {
				message := ""
				for _, d := range event.EventData.Pairs {
					message += d.Value
				}
				event.Message = message
			}
			events = append(events, event)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, errors.Wrap(err, "could not fetch events from subscription")
	}
	el.log.Debugln("Event Log: fetched and parsed all events: %d", len(events))

	if len(events) > 0 {
		el.bookmark = events[len(events)-1]
	}

	return events, nil
}
