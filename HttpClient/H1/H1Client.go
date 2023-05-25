package H1

import (
	"bytes"
	"github.com/tauruscorpius/appcommon/Log"
	"io"
	"net/http"
)

func postRetry(url string, reader *bytes.Reader) (error, *http.Response) {
	resp, err := http.Post(url, "application/json", reader)
	if err != nil {
		Log.Debugf("error making request : %v\n", err)
		return err, nil
	}
	return nil, resp
}

func PostH1(url string, reader *bytes.Reader, readBody bool) (int, string, error) {
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
