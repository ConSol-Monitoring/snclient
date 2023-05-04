package eventlog

import (
	"encoding/xml"
	"fmt"

	"github.com/elastic/beats/v7/libbeat/logp"
	evsys "github.com/elastic/beats/v7/winlogbeat/sys/winevent"
	"github.com/elastic/beats/v7/winlogbeat/sys/wineventlog"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

type EventLog struct {
	LogChannel string
	TimeDiff   uint64

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

func queryStringFromChannels(channels []string, timeDiffSeconds uint64) (string, error) {
	if len(channels) < 1 {
		return "", fmt.Errorf("missing channel for event log query")
	}

	var selects []xmlEventQuerySelect
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
		return "", err
	}
	return string(data), nil
}

func (e *EventLog) prepare() (string, wineventlog.EvtHandle, wineventlog.EvtSubscribeFlag, error) {
	var (
		err      error
		bookmark wineventlog.EvtHandle
		flags    = wineventlog.EvtSubscribeStartAtOldestRecord
	)
	log.Debugln("Event Log: prepare query")

	var timeDiff uint64 = 3600
	if e.TimeDiff > 0 {
		timeDiff = e.TimeDiff
	}

	if e.bookmark != nil {
		timeDiff = 0
		bookmark, err = wineventlog.CreateBookmarkFromRecordID(e.bookmark.Channel, e.bookmark.RecordID)
		if err != nil {
			log.Errorln(errors.Wrap(err, "could not create bookmark"))
			bookmark = 0
		} else {
			flags = wineventlog.EvtSubscribeStartAfterBookmark
		}
	}

	query, err := queryStringFromChannels([]string{e.LogChannel}, timeDiff)
	if err != nil {
		if bookmark != 0 {
			_ = bookmark.Close()
		}
		return "", 0, 0, errors.Wrap(err, "could not create query xml")
	}
	return query, bookmark, flags, nil
}

func (e *EventLog) Query() ([]*evsys.Event, error) {
	query, bookmark, flags, err := e.prepare()
	if err != nil {
		return nil, err
	}
	if bookmark != 0 {
		defer bookmark.Close()
	}

	log.Debugln("Event Log: create windows event handle")
	signalHandle, err := windows.CreateEvent(nil, 1, 1, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not create windows event handle")
	}
	defer windows.CloseHandle(signalHandle)

	log.Debugln("Event Log: subscribe to windows event log with query: ", query)
	subscription, err := wineventlog.Subscribe(0, signalHandle, "", query, bookmark, flags)
	if err != nil {
		return nil, errors.Wrap(err, "could not subscribe to event log")
	}
	defer subscription.Close()

	log.Debugln("Event Log: fetch event log handles")
	iter, err := wineventlog.NewEventIterator(wineventlog.WithSubscription(subscription))
	if err != nil {
		return nil, errors.Wrap(err, "could not create event iterator from subscription")
	}
	defer iter.Close()

	var events []*evsys.Event

	logger := logp.NewLogger("")
	renderer, err := wineventlog.NewRenderer(0, logger)
	if err != nil {
		return nil, err
	}
	defer renderer.Close()

	for {
		eventHandle, more := iter.Next()
		if !more {
			break
		}

		if event, err := renderer.Render(eventHandle); err != nil {
			log.Errorln("Event Log: could not render event: ", err)
		} else {
			events = append(events, event)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, errors.Wrap(err, "could not fetch events from subscription")
	}
	log.Debugln("Event Log: fetched and parsed all events: ", len(events))

	if len(events) > 0 {
		e.bookmark = events[len(events)-1]
	}

	return events, nil
}

func Available() (bool, error) {
	return wineventlog.IsAvailable()
}
