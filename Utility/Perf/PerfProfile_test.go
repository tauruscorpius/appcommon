package Perf

import (
	"testing"
	"time"
)

func TestStartPerfProfile(t *testing.T) {
	err := StartPerfProfile(":F2")
	if err != nil {
		t.Error("StartPerfProfile Error: " + err.Error())
		return
	}
	time.Sleep(30 * time.Second)
	go func() {
		err := StopPerfProfile()
		if err != nil {
			t.Error("StopPerfProfile Error" + err.Error())
			return
		}
	}()
	time.Sleep(10 * time.Second)
}
