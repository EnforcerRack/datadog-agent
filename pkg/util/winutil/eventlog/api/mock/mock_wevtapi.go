// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//go:build windows
// +build windows

package mock

import (
	"bytes"
	"fmt"
	"text/template"

    evtapi "github.com/DataDog/datadog-agent/pkg/util/winutil/eventlog/api"

	"golang.org/x/sys/windows"
)

type MockWindowsEventLogAPI struct {
	eventLogs map[string]*mockEventLog

	subscriptionHandleCount uint
	subscriptions map[evtapi.EventResultSetHandle]*mockSubscription

	eventRecordHandleCount uint
	eventHandles map[evtapi.EventRecordHandle]*mockEventRecord
}

type mockEventLog struct {
	name string
	events []*mockEventRecord
}

type mockSubscription struct {
	channel string
	query string
	handle evtapi.EventResultSetHandle

	nextEvent uint
}

type mockEventRecord struct {
	handle evtapi.EventRecordHandle

	eventLog string

	// Must be exported so template can render them
	EventID uint
	Source string
	Data string
}

func NewMockWindowsEventLogAPI() *MockWindowsEventLogAPI {
	var api MockWindowsEventLogAPI

	api.subscriptions = make(map[evtapi.EventResultSetHandle]*mockSubscription)
	// invalid handle
	api.subscriptions[0] = nil

	api.eventLogs = make(map[string]*mockEventLog)

	api.eventHandles = make(map[evtapi.EventRecordHandle]*mockEventRecord)
	// invalid handle
	api.eventHandles[0] = nil

	return &api
}

func newMockEventLog(name string) *mockEventLog {
	var e mockEventLog
	e.name = name
	return &e
}

func newMockSubscription(channel string, query string) *mockSubscription {
	var s mockSubscription
	s.channel = channel
	s.query = query
	return &s
}

func newMockEventRecord(eventID uint, source string, data string) *mockEventRecord {
	var e mockEventRecord
	e.EventID = eventID
	e.Source = source
	e.Data = data
	return &e
}

//
// Mock helpers
//
func (api *MockWindowsEventLogAPI) AddEventLog(name string) error {
	// does it exist
	_, err := api.getMockEventLog(name)
	if err == nil {
		return fmt.Errorf("Event log %v already exists", name)
	}

	api.addMockEventLog(newMockEventLog(name))
	return nil
}

func (api *MockWindowsEventLogAPI) GenerateEvents(eventLogName string, numEvents uint) error {
	// Get event log
	eventLog, err := api.getMockEventLog(eventLogName)
	if err != nil {
		return err
	}

	// Add junk events
	for i := uint(0); i < numEvents; i+=1 {
		event := newMockEventRecord(1000, "testchannel", "testdata")
		api.addEventRecord(event)
		eventLog.addEventRecord(event)
	}

	return nil
}

//
// internal mock functions
//
func (api *MockWindowsEventLogAPI) addSubscription(sub *mockSubscription) {
	api.subscriptionHandleCount += 1
	h := api.subscriptionHandleCount
	sub.handle = evtapi.EventResultSetHandle(h)
	api.subscriptions[sub.handle] = sub
}

func (api *MockWindowsEventLogAPI) addEventRecord(event *mockEventRecord) {
	api.eventRecordHandleCount += 1
	h := api.eventRecordHandleCount
	event.handle = evtapi.EventRecordHandle(h)
	api.eventHandles[event.handle] = event
}

func (api *MockWindowsEventLogAPI) getMockSubscriptionByHandle(subHandle evtapi.EventResultSetHandle) (*mockSubscription, error) {
	v, ok := api.subscriptions[subHandle]
	if !ok {
		return nil, fmt.Errorf("Subscription not found: %#x", subHandle)
	}
	return v, nil
}

func (api *MockWindowsEventLogAPI) getMockEventRecordByHandle(eventHandle evtapi.EventRecordHandle) (*mockEventRecord, error) {
	v, ok := api.eventHandles[eventHandle]
	if !ok {
		return nil, fmt.Errorf("Event not found: %#x", eventHandle)
	}
	return v, nil
}

func (api *MockWindowsEventLogAPI) getMockEventLog(name string) (*mockEventLog, error) {
	v, ok := api.eventLogs[name]
	if !ok {
		return nil, fmt.Errorf("Event log %v not found", name)
	}
	return v, nil
}

func (api *MockWindowsEventLogAPI) addMockEventLog(eventLog *mockEventLog) {
	api.eventLogs[eventLog.name] = eventLog
}

func (e *mockEventLog) addEventRecord(event *mockEventRecord) {
	e.events = append(e.events, event)
	event.eventLog = e.name
}

//
// Mock Windows APIs
//
func (api *MockWindowsEventLogAPI) EvtSubscribe(
	SignalEvent evtapi.WaitEventHandle,
	ChannelPath string,
	Query string,
	Bookmark evtapi.EventBookmarkHandle,
	Flags uint) (evtapi.EventResultSetHandle, error) {

	if Query != "" && Query != "*" {
		return evtapi.EventResultSetHandle(0), fmt.Errorf("Mock does not support query syntax")
	}

	// ensure channel exists
	_, err := api.getMockEventLog(ChannelPath)
	if err != nil {
		return evtapi.EventResultSetHandle(0), err
	}

	// create sub
	sub := newMockSubscription(ChannelPath, Query)
	api.addSubscription(sub)
	return sub.handle, nil
}

func (api *MockWindowsEventLogAPI) EvtNext(
	Session evtapi.EventResultSetHandle,
	EventsArray []evtapi.EventRecordHandle,
	EventsSize uint,
	Timeout uint) ([]evtapi.EventRecordHandle, error) {

	// get subscription
	sub, err := api.getMockSubscriptionByHandle(Session)
	if err != nil {
		return nil, err
	}

	// get event log
	eventLog, err := api.getMockEventLog(sub.channel)
	if err != nil {
		return nil, err
	}

	// is event log empty
	if len(eventLog.events) == 0 {
		return nil, windows.ERROR_NO_MORE_ITEMS
	}
	// if we are at end
	if sub.nextEvent >= uint(len(eventLog.events)) {
		return nil, windows.ERROR_NO_MORE_ITEMS
	}

	// get next events from log
	end := sub.nextEvent+EventsSize
	if end > uint(len(eventLog.events)) {
		end = uint(len(eventLog.events))
	}
	events := eventLog.events[sub.nextEvent:end]
	eventHandles := make([]evtapi.EventRecordHandle, len(events))
	for i, e := range events {
		eventHandles[i] = e.handle
	}
	sub.nextEvent = end

	return eventHandles, nil
}

func (api *MockWindowsEventLogAPI) EvtClose(h windows.Handle) {
	// nothing to do
}

// EvtRenderEventXmlText renders EvtRenderEventXml
// https://learn.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtrender
func (api *MockWindowsEventLogAPI) EvtRenderEventXml(Fragment evtapi.EventRecordHandle) ([]uint16, error) {
	// get event object
	event, err := api.getMockEventRecordByHandle(Fragment)
	if err != nil {
		return nil, err
	}

	// Format event
	tstr := `<Event xmlns="http://scemas.microsoft.com/win/2004/08/events/event">
  <System>
	<EventID>{{ .EventID }}</EventID>
	<Channel>{{ .Source }}</Channel>
  </System>
  <EventData>
    <Data>{{ .Data }}</Data>
  </EventData>
</Event>
`
	t, err := template.New("eventRenderXML").Parse(tstr)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)

	err = t.Execute(buf, event)
	if err != nil {
		return nil, err
	}

	// convert from utf-8 to utf-16
	res, err := windows.UTF16FromString(buf.String())
	if err != nil {
		return nil, err
	}

	return res, nil
}

// EvtRenderEventXmlText renders EvtRenderEventBookmark
// https://learn.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtrender
func (api *MockWindowsEventLogAPI) EvtRenderBookmark(Fragment evtapi.EventBookmarkHandle) ([]uint16, error) {
	return nil, fmt.Errorf("not implemented")
}
