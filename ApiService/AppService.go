package ApiService

import (
	"github.com/tauruscorpius/appcommon/Log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type AppService struct {
	mapping []PathMapping
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

func (t *AppService) StartHttpApi(listenAddress string, addrAny bool) {
	muxInstance := createHttpMux(t.mapping)
	Log.Criticalf("using h2 for http2, listen @ [%s]\n", listenAddress)
	go func() {
		if addrAny {
			splitAddr := strings.Split(listenAddress, ":")
			if len(splitAddr) >= 2 {
				listenAddress = ":" + splitAddr[len(splitAddr)-1]
				Log.Criticalf("listen any address bind @ [%s]\n", listenAddress)
			}
		}
		_ = t.startServer(listenAddress, muxInstance)
	}()
}

func (t *AppService) startServer(listenAddress string, mux http.Handler) error {
	server := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}
	homeDir := os.Getenv("HOME")
	certKey := homeDir + string(os.PathSeparator) + "/etc/pem/server.key"
	certPem := homeDir + string(os.PathSeparator) + "/etc/pem/server.crt"
	Log.Criticalf("Http API Listen @ %s, cert [key=%s, pem=%s]\n", listenAddress, certKey, certPem)
	err := server.ListenAndServeTLS(certPem, certKey)
	if err != nil {
		Log.Errorf("ListenAndServeTLS failed, error : %v\n", err)
		os.Exit(-1)
	}
	return err
}
