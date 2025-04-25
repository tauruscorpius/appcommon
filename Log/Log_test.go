package Log

import (
	"testing"
	"time"
)

func TestLogOut(t *testing.T) {
	SetOutput("test_LogOut")
	for i := 0; i < 10; i++ {
		Criticalf("test")
		time.Sleep(time.Second)
	}
	CloseOutput()
	time.Sleep(time.Second)
}
