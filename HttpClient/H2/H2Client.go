package H2

import (
	"bytes"
	"crypto/tls"
	"github.com/tauruscorpius/appcommon/Log"
	"golang.org/x/net/http2"
	"io"
	"net/http"
	"sync"
	"time"
)

var (
	http2ClientOnce   sync.Once
	http2ClientServer *http.Client
)

func getClientInstance() *http.Client {
	http2ClientOnce.Do(func() {
		http2ClientTr := &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost: 500,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
		}
		http2.ConfigureTransport(http2ClientTr) // important : enable http2 transport feature
		http2ClientServer = &http.Client{Timeout: time.Second * 10, Transport: http2ClientTr}
	})
	return http2ClientServer
}

func postRetry(url string, reader *bytes.Reader) (error, *http.Response) {
	resp, err := getClientInstance().Post(url, "application/json", reader)
	if err != nil {
		Log.Debugf("error making request : %v\n", err)
		return err, nil
	}
	return nil, resp
}

func PostH2(url string, reader *bytes.Reader, readBody bool) (int, string, error) {
	err, resp := postRetry(url, reader)
	if err != nil {
		err, resp = postRetry(url, reader)
	}
	if err != nil {
		Log.Debugf("error making request : %v\n", err)
		return 0, "", err
	}
	if readBody {
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body), nil
	}
	resp.Body.Close()
	return resp.StatusCode, "", nil
}
