package ExitHandler

import (
	"context"
	"fmt"
	"github.com/tauruscorpius/appcommon/Log"
	"github.com/tauruscorpius/appcommon/Lookup/LookupArgs"
	"github.com/tauruscorpius/appcommon/Utility/Stack"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type SystemStatus int

const (
	SystemInRunning SystemStatus = iota
	SystemExiting
	SystemAtExitFunc
	SystemOutExitFunc
	SystemExited
)

func (t SystemStatus) String() string {
	switch t {
	case SystemInRunning:
		return "SystemInRunning"
	case SystemExiting:
		return "SystemExiting"
	case SystemAtExitFunc:
		return "SystemAtExitFunc"
	case SystemOutExitFunc:
		return "SystemOutExitFunc"
	case SystemExited:
		return "SystemExited"
	default:
		return "SystemStatus Unknown"
	}
}

type ExitProcHandler struct {
	rw         sync.RWMutex
	Status     atomic.Pointer[SystemStatus]
	AppContext context.Context
	ExitFunc   func()
	handler    []func() bool
}

func (t *ExitProcHandler) Init() {
	t.SetStatus(SystemInRunning)
	t.AppContext, t.ExitFunc = context.WithCancel(context.Background())
}

func (t *ExitProcHandler) GetSystemStatus() SystemStatus {
	return *t.Status.Load()
}

func (t *ExitProcHandler) SetExitFlag() {
	t.SetStatus(SystemExiting)
	t.ExitFunc()
}

func (t *ExitProcHandler) Add(a func() bool) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.handler = append(t.handler, a)
}

func (t *ExitProcHandler) SetStatus(s SystemStatus) {
	old := t.Status.Swap(&s)
	var str string
	if old != nil {
		str = fmt.Sprintf("%v", *old)
	} else {
		str = "nil"
	}
	fmt.Fprintf(os.Stdout, "%s System Status Change : %s -> %v\n", time.Now().String(), str, s)

}

func (t *ExitProcHandler) Execute(op chan bool) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.SetStatus(SystemAtExitFunc)

	for _, v := range t.handler {
		v()
	}

	t.SetStatus(SystemOutExitFunc)

	op <- true
	return
}

var (
	once         sync.Once
	exitExecutor *ExitProcHandler
)

func GetExitFuncChain() *ExitProcHandler {
	once.Do(func() {
		exitExecutor = &ExitProcHandler{}
	})
	return exitExecutor
}

func init() {
	sigNotify()
}

func sigNotify() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	GetExitFuncChain().Init()
	go sigCallback(sigs)
	signal.Ignore(syscall.SIGPIPE)
}

func sigCallback(sigs chan os.Signal) {
	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		Log.Criticalf("Receive Signal: %s\n", sig)
		_ = os.Stdout.Close()
		GetExitFuncChain().SetExitFlag()
		go GetExitFuncChain().Execute(op)
		start := time.Now()
		wait := time.After(2 * time.Second)
		select {
		case <-wait:
			Log.Errorf("Execute Exit Func Chain List Timeout, Exit System Right Now\n")
		case m := <-op:
			if time.Since(start).Seconds() < 1 {
				time.Sleep(time.Second)
			}
			Log.Criticalf("Execute Exit Fun Chain List Result : %v\n", m)
		}
		Log.CloseOutput()
		app := LookupArgs.GetLookupAppArgs().Identifier
		_ = Stack.DumpAppStack(app, true)
		GetExitFuncChain().SetStatus(SystemExited)
		os.Exit(0)
	}()
}
