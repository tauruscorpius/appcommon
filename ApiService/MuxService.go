package ApiService

import "net/http"

type PathMapping struct {
	Path string
	Call func(w http.ResponseWriter, r *http.Request)
}

var (
	mm map[string]func(w http.ResponseWriter, r *http.Request)
)

func createHttpMux(mapping []PathMapping) http.HandlerFunc {
	mm = make(map[string]func(w http.ResponseWriter, r *http.Request))
	for _, v := range mapping {
		mm[v.Path] = v.Call
	}
	f := func(w http.ResponseWriter, r *http.Request) {
		d, o := mm[r.URL.Path]
		if o {
			d(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
	return f
}
