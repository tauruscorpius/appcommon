package Log

import (
	"testing"
	"time"
)

func TestLogOut(t *testing.T) {
	SetOutput("test_LogOut")
	time.Sleep(time.Second * 2)
	for i := 0; i < 10; i++ {
		Criticalf("test")
		time.Sleep(time.Second)
	}
}
