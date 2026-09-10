package H1

import (
	"bytes"
	"io"
	"net/http"

	"github.com/tauruscorpius/appcommon/Log"
)

func postOnce(url string, reader *bytes.Reader) (error, *http.Response) {
	_, _ = reader.Seek(0, io.SeekStart)
	resp, err := http.Post(url, "application/json", reader)
	if err != nil {
		Log.Debugf("error making request : %v\n", err)
		return err, nil
	}
	return nil, resp
}

func PostH1(url string, reader *bytes.Reader, readBody bool) (int, string, error) {
	err, resp := postOnce(url, reader)
	if err != nil {
		Log.Debugf("error making request : %v\n", err)
		return 0, "", err
	}
	defer resp.Body.Close()
	if readBody {
		body, errBody := io.ReadAll(resp.Body)
		if errBody != nil {
			return resp.StatusCode, "", errBody
		}
		return resp.StatusCode, string(body), nil
	}
	return resp.StatusCode, "", nil
}
