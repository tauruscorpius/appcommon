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
	NodeDumpAppStack  SysEventId = "dumpAppStack"
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
	rw       sync.RWMutex
	hookFunc map[string][]func(args []string) bool
}

func (t *EventRequestHook) Init() bool {
	t.hookFunc = make(map[string][]func(args []string) bool)
	return true
}

func (t *EventRequestHook) RegisterHook(oid string, f func(args []string) bool) {
	t.rw.Lock()
	defer t.rw.Unlock()

	_, o := t.hookFunc[oid]
	if o {
		t.hookFunc[oid] = append(t.hookFunc[oid], f)
		return
	}
	t.hookFunc[oid] = []func(args []string) bool{f}
}

func (t *EventRequestHook) EventRequest(eventId string, eventArgs []string) bool {
	fn, exist := t.hookFunc[eventId]
	if exist {
		r := true
		Log.Criticalf("Event Id [%s] has %d hook(s)\n", eventId, len(fn))
		for i, v := range fn {
			e := v(eventArgs)
			Log.Criticalf("Event Id [%s] exec %d hook; result : %v\n", eventId, i+1, e)
			r = r && e
		}
		return r
	}
	Log.Criticalf("Sys Event Id [%s] take no action\n", eventId)
	return false
}
