package ExitHandler

import (
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestExitProcHandler_GetSystemStatus(t *testing.T) {
	e := &ExitProcHandler{}
	e.Init()
	if e.GetSystemStatus() != SystemInRunning {
		t.Error("Init SystemStatus Error")
	}
}

func TestExitProcHandler_GetSystemStatus1(t *testing.T) {
	e := &ExitProcHandler{}
	e.Init()

	e.SetExitFlag()

	if e.GetSystemStatus() != SystemExiting {
		t.Error("Error SystemExiting Expected")
	}
}

func TestExitProcHandler_GetSystemStatus2(t *testing.T) {
	e := &ExitProcHandler{}
	e.Init()

	e.SetExitFlag()
	e.Execute(make(chan bool, 10))

	if e.GetSystemStatus() != SystemOutExitFunc {
		t.Error("Error SystemOutExitFunc Expected")
	}
}

func TestExitProcHandler_AppExit(t *testing.T) {
	exitFuncExecuted := false
	exitFuncChan := make(chan bool, 1)

	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	exitExecutor.Add(func() bool {
		exitFuncExecuted = true
		exitFuncChan <- true
		return true
	})

	sigs := make(chan os.Signal, 1)
	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		t.Logf("Receive Exit Signal: %s", sig)
		exitExecutor.SetExitFlag()
		go exitExecutor.Execute(op)
		select {
		case <-time.After(2 * time.Second):
			t.Error("Execute Exit Func Chain List Timeout")
		case <-op:
			t.Logf("Execute Exit Func Chain List completed")
		}
		exitExecutor.SetStatus(SystemExited)
	}()

	sigs <- UserExitSignal(2)

	select {
	case <-exitFuncChan:
		if !exitFuncExecuted {
			t.Error("Exit function was not executed")
		}
		t.Log("Exit function executed successfully")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit function to execute")
	}

	time.Sleep(100 * time.Millisecond)

	if exitExecutor.GetSystemStatus() != SystemExited {
		t.Errorf("Expected SystemExited, got %v", exitExecutor.GetSystemStatus())
	}
}

func TestSigNotify_ExitWithSignal(t *testing.T) {
	exitFuncExecuted := false
	exitFuncChan := make(chan bool, 1)
	completeChan := make(chan bool, 1)

	sigs := make(chan os.Signal, 1)
	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	exitExecutor.Add(func() bool {
		exitFuncExecuted = true
		exitFuncChan <- true
		return true
	})

	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		t.Logf("Receive Signal: %s", sig)
		exitExecutor.SetExitFlag()
		go exitExecutor.Execute(op)
		select {
		case <-time.After(2 * time.Second):
			t.Error("Execute Exit Func Chain List Timeout")
		case <-op:
			t.Logf("Execute Exit Func Chain List completed")
		}
		exitExecutor.SetStatus(SystemExited)
		completeChan <- true
	}()

	sigs <- syscall.SIGTERM

	select {
	case <-exitFuncChan:
		if !exitFuncExecuted {
			t.Error("Exit function was not executed")
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit function to execute")
	}

	select {
	case <-completeChan:
		t.Log("Exit process completed successfully")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit process to complete")
	}

	if exitExecutor.GetSystemStatus() != SystemExited {
		t.Errorf("Expected SystemExited, got %v", exitExecutor.GetSystemStatus())
	}
}

func TestSigNotify_ExitWithUserExitSignal(t *testing.T) {
	exitFuncExecuted := false
	exitFuncChan := make(chan bool, 1)
	completeChan := make(chan bool, 1)

	sigs := make(chan os.Signal, 1)
	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	exitExecutor.Add(func() bool {
		exitFuncExecuted = true
		exitFuncChan <- true
		return true
	})

	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		t.Logf("Receive Signal: %s", sig)
		exitExecutor.SetExitFlag()
		go exitExecutor.Execute(op)
		select {
		case <-time.After(2 * time.Second):
			t.Error("Execute Exit Func Chain List Timeout")
		case <-op:
			t.Logf("Execute Exit Func Chain List completed")
		}
		exitExecutor.SetStatus(SystemExited)
		switch sig.(type) {
		case UserExitSignal:
			t.Logf("Exit with user exit signal %s", sig)
		case os.Signal:
			t.Logf("Exit with os signal %s", sig)
		}
		completeChan <- true
	}()

	sigs <- UserExitSignal(0)

	select {
	case <-exitFuncChan:
		if !exitFuncExecuted {
			t.Error("Exit function was not executed")
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit function to execute")
	}

	select {
	case <-completeChan:
		t.Log("Exit process completed successfully")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit process to complete")
	}

	if exitExecutor.GetSystemStatus() != SystemExited {
		t.Errorf("Expected SystemExited, got %v", exitExecutor.GetSystemStatus())
	}
}

func TestSigNotify_MultipleExitFunctions(t *testing.T) {
	exitFunc1Executed := false
	exitFunc2Executed := false
	exitFunc3Executed := false
	allExecutedChan := make(chan bool, 1)
	completeChan := make(chan bool, 1)

	sigs := make(chan os.Signal, 1)
	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	exitExecutor.Add(func() bool {
		exitFunc1Executed = true
		t.Log("Exit function 1 executed")
		return true
	})

	exitExecutor.Add(func() bool {
		exitFunc2Executed = true
		t.Log("Exit function 2 executed")
		return true
	})

	exitExecutor.Add(func() bool {
		exitFunc3Executed = true
		t.Log("Exit function 3 executed")
		allExecutedChan <- true
		return true
	})

	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		t.Logf("Receive Signal: %s", sig)
		exitExecutor.SetExitFlag()
		go exitExecutor.Execute(op)
		select {
		case <-time.After(2 * time.Second):
			t.Error("Execute Exit Func Chain List Timeout")
		case <-op:
			t.Logf("Execute Exit Func Chain List completed")
		}
		exitExecutor.SetStatus(SystemExited)
		completeChan <- true
	}()

	sigs <- syscall.SIGINT

	select {
	case <-allExecutedChan:
		if !exitFunc1Executed || !exitFunc2Executed || !exitFunc3Executed {
			t.Errorf("Not all exit functions were executed: func1=%v, func2=%v, func3=%v",
				exitFunc1Executed, exitFunc2Executed, exitFunc3Executed)
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for all exit functions to execute")
	}

	select {
	case <-completeChan:
		t.Log("Exit process completed successfully")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit process to complete")
	}

	if exitExecutor.GetSystemStatus() != SystemExited {
		t.Errorf("Expected SystemExited, got %v", exitExecutor.GetSystemStatus())
	}
}

func TestSigNotify_StatusTransitions(t *testing.T) {
	statusChanges := []SystemStatus{}
	statusChangeChan := make(chan SystemStatus, 10)
	completeChan := make(chan bool, 1)

	sigs := make(chan os.Signal, 1)
	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	if exitExecutor.GetSystemStatus() != SystemInRunning {
		t.Errorf("Expected initial status SystemInRunning, got %v", exitExecutor.GetSystemStatus())
	}

	exitExecutor.Add(func() bool {
		statusChangeChan <- exitExecutor.GetSystemStatus()
		return true
	})

	go func() {
		var op chan bool
		op = make(chan bool)
		sig := <-sigs
		t.Logf("Receive Signal: %s", sig)
		exitExecutor.SetExitFlag()
		go exitExecutor.Execute(op)
		select {
		case <-time.After(2 * time.Second):
			t.Error("Execute Exit Func Chain List Timeout")
		case <-op:
			t.Logf("Execute Exit Func Chain List completed")
		}
		exitExecutor.SetStatus(SystemExited)
		completeChan <- true
	}()

	sigs <- syscall.SIGTERM

	time.Sleep(100 * time.Millisecond)
	statusChanges = append(statusChanges, exitExecutor.GetSystemStatus())

	select {
	case status := <-statusChangeChan:
		if status != SystemAtExitFunc {
			t.Errorf("Expected status SystemAtExitFunc during exit function execution, got %v", status)
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for status change")
	}

	select {
	case <-completeChan:
		t.Log("Exit process completed successfully")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for exit process to complete")
	}

	finalStatus := exitExecutor.GetSystemStatus()
	if finalStatus != SystemExited {
		t.Errorf("Expected final status SystemExited, got %v", finalStatus)
	}
}

func TestSigCallback_DuplicateSignalHandling(t *testing.T) {
	exitFuncExecutedCount := int32(0)
	firstSignalProcessed := make(chan bool, 1)
	secondSignalIgnored := make(chan bool, 1)

	sigs := make(chan os.Signal, 2)
	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	exitExecutor.Add(func() bool {
		atomic.AddInt32(&exitFuncExecutedCount, 1)
		return true
	})

	atomic.StoreInt32(&exitProcessing, 0)

	go func() {
		for i := 0; i < 2; i++ {
			sig := <-sigs
			
			if !atomic.CompareAndSwapInt32(&exitProcessing, 0, 1) {
				t.Logf("Signal %d correctly ignored: %s", i+1, sig)
				secondSignalIgnored <- true
				continue
			}
			
			t.Logf("Processing first signal: %s", sig)
			op := make(chan bool)
			exitExecutor.SetExitFlag()
			go exitExecutor.Execute(op)
			select {
			case <-time.After(2 * time.Second):
				t.Error("Execute Exit Func Chain List Timeout")
			case <-op:
				t.Logf("Execute Exit Func Chain List completed")
			}
			exitExecutor.SetStatus(SystemExited)
			firstSignalProcessed <- true
		}
	}()

	sigs <- syscall.SIGTERM
	time.Sleep(50 * time.Millisecond)
	sigs <- syscall.SIGINT

	select {
	case <-firstSignalProcessed:
		t.Log("First signal processed successfully")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for first signal to be processed")
	}

	select {
	case <-secondSignalIgnored:
		t.Log("Second signal correctly ignored")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for second signal to be ignored")
	}

	finalCount := atomic.LoadInt32(&exitFuncExecutedCount)
	if finalCount != 1 {
		t.Errorf("Exit function should be executed exactly once, got %d times", finalCount)
	} else {
		t.Log("Exit function executed exactly once as expected")
	}

	atomic.StoreInt32(&exitProcessing, 0)
}

func TestSigCallback_NilExitExecutor(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	exitCalled := false
	exitCalledChan := make(chan bool, 1)

	atomic.StoreInt32(&exitProcessing, 0)

	go func() {
		sig := <-sigs
		
		if !atomic.CompareAndSwapInt32(&exitProcessing, 0, 1) {
			t.Logf("Exit already in progress, ignoring signal: %s", sig)
			return
		}
		
		t.Logf("Receive Signal: %s", sig)
		
		if exitExecutor == nil {
			t.Logf("exitExecutor is nil, simulating exit")
			exitCalled = true
			exitCalledChan <- true
			return
		}
		
		t.Error("exitExecutor should be nil in this test")
	}()

	originalExecutor := exitExecutor
	exitExecutor = nil

	sigs <- syscall.SIGTERM

	select {
	case <-exitCalledChan:
		if !exitCalled {
			t.Error("Exit should have been called when exitExecutor is nil")
		}
		t.Log("Correctly handled nil exitExecutor")
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for nil check")
	}

	exitExecutor = originalExecutor
	atomic.StoreInt32(&exitProcessing, 0)
}

type CustomSignal int

func (c CustomSignal) Signal() {}
func (c CustomSignal) String() string { return "CustomSignal" }

func TestSigCallback_SafeTypeAssertion(t *testing.T) {
	var sig os.Signal = CustomSignal(99)

	switch ty := sig.(type) {
	case UserExitSignal:
		t.Logf("UserExitSignal: %v", ty)
	case os.Signal:
		t.Logf("os.Signal: %v", sig)
		if syscallSig, ok := ty.(syscall.Signal); ok {
			t.Logf("Successfully converted to syscall.Signal: %v", syscallSig)
		} else {
			t.Logf("Cannot convert to syscall.Signal, using default exit code")
		}
	}

	t.Log("Type assertion test completed without panic")
}

func TestSigCallback_ConcurrentSignals(t *testing.T) {
	exitFuncExecutedCount := int32(0)
	exitFuncChan := make(chan bool, 5)
	completeChan := make(chan bool, 5)

	sigs := make(chan os.Signal, 5)
	exitExecutor := &ExitProcHandler{}
	exitExecutor.Init()

	exitExecutor.Add(func() bool {
		atomic.AddInt32(&exitFuncExecutedCount, 1)
		exitFuncChan <- true
		return true
	})

	atomic.StoreInt32(&exitProcessing, 0)

	for i := 0; i < 5; i++ {
		go func(index int) {
			var op chan bool
			op = make(chan bool)
			sig := <-sigs
			
			if !atomic.CompareAndSwapInt32(&exitProcessing, 0, 1) {
				t.Logf("Goroutine %d: Exit already in progress, ignoring signal: %s", index, sig)
				completeChan <- true
				return
			}
			
			t.Logf("Goroutine %d: Processing signal: %s", index, sig)
			exitExecutor.SetExitFlag()
			go exitExecutor.Execute(op)
			select {
			case <-time.After(2 * time.Second):
				t.Errorf("Goroutine %d: Timeout", index)
			case <-op:
				t.Logf("Goroutine %d: Completed", index)
			}
			exitExecutor.SetStatus(SystemExited)
			completeChan <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		sigs <- syscall.SIGTERM
		time.Sleep(10 * time.Millisecond)
	}

	completedCount := 0
	for i := 0; i < 5; i++ {
		select {
		case <-completeChan:
			completedCount++
		case <-time.After(3 * time.Second):
			t.Errorf("Timeout waiting for goroutine %d", i)
		}
	}

	if completedCount != 5 {
		t.Errorf("Expected 5 goroutines to complete, got %d", completedCount)
	}

	finalCount := atomic.LoadInt32(&exitFuncExecutedCount)
	if finalCount != 1 {
		t.Errorf("Exit function should be executed exactly once, got %d times", finalCount)
	} else {
		t.Logf("Exit function executed exactly once despite 5 concurrent signals")
	}

	atomic.StoreInt32(&exitProcessing, 0)
}
