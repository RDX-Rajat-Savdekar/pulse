package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// all: keeps the Astro _astro asset directory, which embed would otherwise skip.
//
//go:embed all:dist
var files embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
