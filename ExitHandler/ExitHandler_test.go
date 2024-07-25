package ExitHandler

import "testing"

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
