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

type UserExitSignal int

func (t UserExitSignal) String() string {
	return "UserExitSignal"
}

func (t UserExitSignal) Signal() {}

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
	fmt.Printf("%s System Status Change : %s -> %v\n", time.Now().String(), str, s)

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
	userExitFunc = func(code int) {}
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
	finished := make(chan bool)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	GetExitFuncChain().Init()
	go sigCallback(sigs, finished)
	signal.Ignore(syscall.SIGPIPE)
	userExitFunc = func(code int) {
		sigs <- syscall.SIGTERM //UserExitSignal(-1)
		<-finished
	}
}

func sigCallback(sigs chan os.Signal, finished chan bool) {
	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		Log.Criticalf("Receive Signal: %s\n", sig)
		exitExecutor.SetExitFlag()
		go exitExecutor.Execute(op)
		start := time.Now()
		wait := time.After(2 * time.Second)
		select {
		case <-wait:
			Log.Errorf("Execute Exit Func Chain List Timeout, Exit System Right Now\n")
		case m := <-op:
			now := time.Now()
			if now.Sub(start) < time.Second {
				time.Sleep(time.Second - now.Sub(start))
			}
			Log.Criticalf("Execute Exit Fun Chain List Result : %v\n", m)
		}
		Log.CloseOutput()
		lookupArgs := LookupArgs.GetLookupAppArgs()
		app := lookupArgs.AppName + "." + lookupArgs.Identifier
		_ = Stack.DumpAppStack(app, true)
		exitExecutor.SetStatus(SystemExited)
		finished <- true
		switch ty := sig.(type) {
		case UserExitSignal:
			fmt.Printf("exit with user signal %s\n", sig)
			os.Exit(int(ty))
		case os.Signal:
			fmt.Printf("exit with signal %s\n", sig)
			os.Exit(int(ty.(syscall.Signal)))
		}
	}()
}

func Exit(code int) {
	userExitFunc(code)
}
