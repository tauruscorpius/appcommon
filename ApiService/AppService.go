package ApiService

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/tauruscorpius/appcommon/ExitHandler"
	"github.com/tauruscorpius/appcommon/Log"
)

type AppService struct {
	mapping []PathMapping
}

type HttpApiStartResult struct {
	Ready <-chan struct{}
	Err   <-chan error
}

func (t HttpApiStartResult) Check() error {
	select {
	case <-t.Ready:
		return nil
	default:
	}

	select {
	case <-t.Ready:
		return nil
	case err, ok := <-t.Err:
		if ok && err != nil {
			return err
		}
		return errors.New("http api start failed without error")
	}
}

var (
	once       sync.Once
	appService *AppService
)

func GetAppService() *AppService {
	once.Do(func() {
		appService = &AppService{}
	})
	return appService
}

func (t *AppService) AddMapping(Path string, Call func(w http.ResponseWriter, r *http.Request)) {
	t.mapping = append(t.mapping, PathMapping{Path: Path, Call: Call})
}

func (t *AppService) MergeMapping(m []PathMapping) {
	t.mapping = append(t.mapping, m...)
}

func (t *AppService) StartHttpApi(listenAddress string, addrAny bool) HttpApiStartResult {
	running := func() bool {
		// system running
		return ExitHandler.GetExitFuncChain().GetSystemStatus() == ExitHandler.SystemInRunning
	}
	muxInstance := createHttpMux(t.mapping, running)
	Log.Criticalf("using h2 for http2, listen @ [%s]\n", listenAddress)
	return t.startServer(listenAddress, addrAny, muxInstance)
}

func (t *AppService) startServer(listenAddress string, addrAny bool, mux http.Handler) HttpApiStartResult {
	ready := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		if addrAny {
			_, port, err := net.SplitHostPort(listenAddress)
			if err != nil {
				errCh <- err
				return
			}
			listenAddress = net.JoinHostPort("", port)
			Log.Criticalf("listen any address bind @ [%s]\n", listenAddress)
		}

		listener, err := net.Listen("tcp", listenAddress)
		if err != nil {
			Log.Errorf("Http API listen failed @ %s, error : %v\n", listenAddress, err)
			errCh <- err
			return
		}

		homeDir := os.Getenv("HOME")
		certKey := homeDir + string(os.PathSeparator) + "/etc/pem/server.key"
		certPem := homeDir + string(os.PathSeparator) + "/etc/pem/server.crt"
		cert, err := tls.LoadX509KeyPair(certPem, certKey)
		if err != nil {
			listener.Close()
			Log.Errorf("Load TLS cert failed, cert [key=%s, pem=%s], error : %v\n", certKey, certPem, err)
			errCh <- err
			return
		}

		server := &http.Server{
			Addr:    listenAddress,
			Handler: mux,
			TLSConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
			},
		}
		Log.Criticalf("Http API Listen @ %s, cert [key=%s, pem=%s]\n", listenAddress, certKey, certPem)
		close(ready)

		if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
			Log.Errorf("ServeTLS failed, error : %v\n", err)
			errCh <- err
		}
	}()

	return HttpApiStartResult{Ready: ready, Err: errCh}
}
