package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed dist
var distFS embed.FS

// RegisterStaticFiles serves the embedded frontend build.
// All routes not matched by the API are served the index.html (SPA fallback).
func RegisterStaticFiles(r *gin.Engine) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem for dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(ctx *gin.Context) {
		// Try to serve the exact file; if not found, return index.html
		path := ctx.Request.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		f, err := sub.Open(path[1:]) // strip leading /
		if err != nil {
			// SPA fallback
			ctx.Request.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(ctx.Writer, ctx.Request)
	})
}
