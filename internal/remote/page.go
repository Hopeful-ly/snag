package remote

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	pageFile  = "remote.html"
	assetsDir = "assets"
)

// Page serves the phone page from the frontend build at "/", its assets, and icon (PNG) at
// "/icon.png", and nothing else of the build. Its CSP allows only its own files and no framing.
func Page(fsys fs.FS, icon []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "use GET", http.StatusMethodNotAllowed)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; "+
			"style-src 'self' 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		switch name := strings.TrimPrefix(r.URL.Path, "/"); {
		case name == "":
			h.Set("Cache-Control", "no-cache") // a new Snag brings a new page
			serveFile(w, r, fsys, pageFile)
		case name == "icon.png":
			h.Set("Content-Type", "image/png")
			h.Set("Cache-Control", "max-age=86400")
			_, _ = w.Write(icon)
		case path.Dir(name) == assetsDir && fs.ValidPath(name):
			h.Set("Cache-Control", "max-age=31536000, immutable") // the names carry a hash
			serveFile(w, r, fsys, name)
		default:
			http.NotFound(w, r)
		}
	})
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	if st, err := fs.Stat(fsys, name); err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, fsys, name)
}
