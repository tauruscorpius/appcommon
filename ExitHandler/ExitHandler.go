package ExitHandler

import (
	"context"
	"github.com/tauruscorpius/appcommon/Log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type SystemStatus int

const (
	SystemInRunning = iota
	SystemExiting
	SystemExited
)

type ExitProcHandler struct {
	rw         sync.RWMutex
	Status     SystemStatus
	AppContext context.Context
	ExitFunc   func()
	handler    []func() bool
}

func (t *ExitProcHandler) Init() {
	t.Status = SystemInRunning
	t.AppContext, t.ExitFunc = context.WithCancel(context.Background())
}

func (t *ExitProcHandler) SetExitFlag() {
	t.ExitFunc()
}

func (t *ExitProcHandler) Add(a func() bool) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.handler = append(t.handler, a)
}

func (t *ExitProcHandler) Execute(op chan bool) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.Status = SystemExiting

	for _, v := range t.handler {
		v()
	}

	t.Status = SystemExited
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
		os.Exit(0)
	}()
}
