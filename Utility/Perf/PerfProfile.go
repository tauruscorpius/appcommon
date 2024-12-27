package Perf

import (
	"errors"
	"github.com/tauruscorpius/appcommon/Log"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

var (
	server *http.Server
	addr   string
	rw     sync.Mutex
)

func CheckPerfProfile() string {
	rw.Lock()
	defer rw.Unlock()

	if server != nil {
		return "not running"
	}

	return "running @" + addr
}

func StartPerfProfile(addr string) error {
	rw.Lock()
	defer rw.Unlock()

	if server != nil {
		return errors.New("server already started @" + addr)
	}

	server = &http.Server{Addr: addr}

	go func() {
		Log.Criticalf("Starting pprof server on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Log.Errorf("pprof server failed: %s", err)
		}
		Log.Criticalf("Exiting pprof server on %s\n", addr)
	}()

	go func() {
		Log.Criticalf("Timing pprof server on %s, it will be automatic closed after 12 hour.\n", addr)
		time.Sleep(12 * time.Hour)
		StopPerfProfile()
	}()

	return nil
}

func StopPerfProfile() error {
	rw.Lock()
	defer rw.Unlock()

	if server == nil {
		return errors.New("pprof server not started")
	}

	Log.Criticalf("Stop pprof server on %s\n", addr)

	if err := server.Close(); err != nil {
		return errors.New("failed to stop pprof server: " + err.Error())
	}

	server = nil
	addr = ""

	return nil
}
