package Perf

import (
	"testing"
	"time"
)

func TestStartPerfProfile(t *testing.T) {
	err := StartPerfProfile(":8080")
	if err != nil {
		t.Error("StartPerfProfile Error")
	}
	time.Sleep(30 * time.Second)
	go func() {
		err := StopPerfProfile()
		if err != nil {
			t.Error("StopPerfProfile Error")
		}
	}()
	time.Sleep(10 * time.Second)
}
