// Package frontend embeds the built web UI (frontend/dist) into the Go binary.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built UI, rooted at dist/. Run `npm run build` in this folder first;
// without it the app starts with an empty page.
func Assets() fs.FS {
	assets, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // only possible if the embed path above is wrong
	}
	return assets
}
