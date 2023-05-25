package HttpClient

import (
	"bytes"
	"github.com/tauruscorpius/appcommon/HttpClient/H1"
	"github.com/tauruscorpius/appcommon/HttpClient/H2"
	"github.com/tauruscorpius/appcommon/Json"
	"github.com/tauruscorpius/appcommon/Log"
	"strings"
)

func PostHx(url string, obj interface{}, readBody bool) (int, string, error) {
	var data []byte
	switch x := obj.(type) {
	case []byte:
		data = x
	case string:
		data = []byte(x)
	default:
		d, e := Json.Marshal(x)
		if e != nil {
			Log.Errorf("PostHx[%s]: object[%+v] marshal failed\n", url, x)
			return 0, "", e
		}
		data = d
	}

	reader := bytes.NewReader(data)

	if strings.HasPrefix(url, "https:") {
		return H2.PostH2(url, reader, readBody)
	}
	return H1.PostH1(url, reader, readBody)
}
