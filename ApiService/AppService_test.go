package ApiService

import (
	"testing"
	"time"
)

func TestStartHttpApi_InvalidListenAddressReturnsError(t *testing.T) {
	svc := &AppService{}
	result := svc.StartHttpApi("127.0.0.1:notaport", false)

	select {
	case err, ok := <-result.Err:
		if !ok || err == nil {
			t.Fatalf("expected listen error, got err=%v ok=%v", err, ok)
		}
	case <-result.Ready:
		t.Fatal("ready should not close when listen address is invalid")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for listen error")
	}
}

func TestHttpApiStartResult_CheckReturnsStartError(t *testing.T) {
	svc := &AppService{}
	result := svc.StartHttpApi("127.0.0.1:notaport", false)

	if err := result.Check(); err == nil {
		t.Fatal("expected start error")
	}
}

func TestStartHttpApi_InvalidAddrAnyAddressReturnsError(t *testing.T) {
	svc := &AppService{}
	result := svc.StartHttpApi("invalid-address", true)

	select {
	case err, ok := <-result.Err:
		if !ok || err == nil {
			t.Fatalf("expected addrAny split error, got err=%v ok=%v", err, ok)
		}
	case <-result.Ready:
		t.Fatal("ready should not close when addrAny address is invalid")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for addrAny error")
	}
}
