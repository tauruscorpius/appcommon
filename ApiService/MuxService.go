package ApiService

import (
	"github.com/tauruscorpius/appcommon/Log"
	"net/http"
)

type PathMapping struct {
	Path string
	Call func(w http.ResponseWriter, r *http.Request)
}

var (
	mm map[string]func(w http.ResponseWriter, r *http.Request)
)

func createHttpMux(mapping []PathMapping, running func() bool) http.HandlerFunc {
	mm = make(map[string]func(w http.ResponseWriter, r *http.Request))
	for _, v := range mapping {
		mm[v.Path] = v.Call
	}
	f := func(w http.ResponseWriter, r *http.Request) {
		if !running() {
			Log.Errorf("System in graceful exit status, all request action locked.")
			w.WriteHeader(http.StatusLocked)
			return
		}
		d, o := mm[r.URL.Path]
		if o {
			d(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
	return f
}
