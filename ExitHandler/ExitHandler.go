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
	once            sync.Once
	exitExecutor    *ExitProcHandler
	userExitFunc    = func(code int) {}
	exitProcessing  int32
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
	userExitFunc = func(code int) {
		sigs <- UserExitSignal(code)
		time.Sleep(5 * time.Second)
		os.Exit(-1)
	}
}

func sigCallback(sigs chan os.Signal) {
	go func() {
		sig := <-sigs
		
		if !atomic.CompareAndSwapInt32(&exitProcessing, 0, 1) {
			Log.Warnf("Exit already in progress, ignoring signal: %s\n", sig)
			return
		}
		
		Log.Criticalf("Receive Signal: %s\n", sig)
		
		if exitExecutor == nil {
			Log.Errorf("exitExecutor is nil, cannot process exit gracefully\n")
			os.Exit(-1)
			return
		}
		
		op := make(chan bool)
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
		
		switch ty := sig.(type) {
		case UserExitSignal:
			fmt.Printf("Exit with user exit signal %s\n", sig)
			os.Exit(int(ty))
		case os.Signal:
			fmt.Printf("Exit with os signal %s\n", sig)
			if syscallSig, ok := ty.(syscall.Signal); ok {
				os.Exit(int(syscallSig))
			} else {
				Log.Errorf("Unknown signal type, exiting with code 1\n")
				os.Exit(1)
			}
		}
	}()
}

func Exit(code int) {
	userExitFunc(code)
}
