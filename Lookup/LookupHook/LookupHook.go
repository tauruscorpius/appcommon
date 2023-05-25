package LookupHook

import (
	"github.com/tauruscorpius/appcommon/Log"
	"sync"
)

type SysEventId string

const (
	NodeSetLogLevel   SysEventId = "setLogLevel"
	NodeCacheRefresh  SysEventId = "cacheRefresh"
	NodeUpdatedNotify SysEventId = "nodeUpdatedNotify"
)

var (
	eventRequestHook *EventRequestHook
	once             sync.Once
)

func GetEventRequest() *EventRequestHook {
	once.Do(func() {
		eventRequestHook = &EventRequestHook{}
		eventRequestHook.Init()
	})
	return eventRequestHook

}

type EventRequestHook struct {
	hookFunc map[string]func(args []string) bool
}

func (t *EventRequestHook) Init() bool {
	t.hookFunc = make(map[string]func(args []string) bool)
	return true
}

func (t *EventRequestHook) RegisterHook(oid string, f func(args []string) bool) {
	t.hookFunc[oid] = f
}

func (t *EventRequestHook) EventRequest(eventId string, eventArgs []string) bool {
	fn, exist := t.hookFunc[eventId]
	if exist {
		return fn(eventArgs)
	}
	Log.Criticalf("Sys Event Id [%s] take no action\n", eventId)
	return false
}
