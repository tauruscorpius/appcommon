package Perf

import (
	"errors"
	"github.com/tauruscorpius/appcommon/Log"
	"github.com/tauruscorpius/appcommon/Utility/UUID"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

type PerfProfServer struct {
	rw     sync.Mutex
	server *http.Server
	addr   string
	id     string
}

var (
	profServer PerfProfServer
)

func CheckPerfProfile() string {
	return profServer.check()
}

func StartPerfProfile(addr string) error {
	if addr == "" {
		return errors.New("invalid address: " + addr)
	}

	return profServer.start(addr)
}

func StopPerfProfile() error {
	return profServer.stop(true)
}

func (t *PerfProfServer) start(addr string) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	if t.server != nil {
		return errors.New("server already started @" + profServer.addr + " id : " + t.id)
	}

	t.id = UUID.GetUid()
	t.server = &http.Server{Addr: addr}
	t.addr = addr

	go func(server *http.Server) {
		id := t.id
		Log.Criticalf("Starting pprof server on %s, id : %s\n", t.addr, id)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Log.Errorf("pprof server failed: %s, id : %s", err, id)
		}
		Log.Criticalf("Exiting pprof server on %s, id : %s\n", t.addr, id)
		t.stop(false)
	}(t.server)

	go func() {
		id := t.id
		start := time.Now()
		Log.Criticalf("Start Timing pprof server on %s, it will be automatic closed after 12 hour, id : %s.\n", t.addr, id)
		for t.server != nil && time.Since(start) < 12*time.Hour {
			time.Sleep(time.Second)
		}
		Log.Criticalf("Exit timing pprof server on %s, id : %s.\n", t.addr, id)
		t.stop(true)
	}()

	return nil
}

func (t *PerfProfServer) check() string {
	t.rw.Lock()
	defer t.rw.Unlock()

	if t.server != nil {
		return "not running"
	}

	return "running @" + profServer.addr + " id : " + t.id
}

func (t *PerfProfServer) stop(stop bool) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	if stop {
		if t.server != nil {
			Log.Criticalf("Stop pprof server on %s, id : %s\n", t.addr, t.id)

			if err := t.server.Close(); err != nil {
				return errors.New("failed to stop pprof server: " + err.Error() + "id :" + t.id)
			}
		}
	}

	t.server = nil
	t.addr = ""
	t.id = ""

	return nil
}
