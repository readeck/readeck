package assets

import (
	"embed"
	"io/fs"
	"net/http"
)

// Assets contains all the static files needed by the app
//
//go:embed www
var Assets embed.FS

// StaticFilesFS returns the assets "www" subfolder as an HTTP
// filesystem.
func StaticFilesFS() http.FileSystem {
	sub, err := fs.Sub(Assets, "www")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
