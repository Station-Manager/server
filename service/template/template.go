package template

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var content embed.FS

func FrontendFS() http.FileSystem {
	pub, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(pub)
}
