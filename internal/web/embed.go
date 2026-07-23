package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the built Vue frontend, falling back to index.html for any
// path that isn't a real static asset so client-side routing works.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	staticFS := http.FS(sub)
	fileServer := http.FileServer(staticFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean(r.URL.Path)
		if f, err := staticFS.Open(cleaned); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
