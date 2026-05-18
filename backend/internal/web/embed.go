// Package web serves the embedded Vue SPA bundle. The production
// frontend build lives in dist/; until that has been produced, dist/
// is empty except for a placeholder and the SPA fallback will surface
// a clear "frontend not built" message.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// Register mounts the SPA at every route not already claimed by the
// API (via r.NoRoute). Static assets are served from dist/, and any
// unknown path falls back to dist/index.html so the Vue router can
// take over (history mode).
func Register(r *gin.Engine) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: failed to scope dist sub-fs: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		// API routes that fell through to NoRoute are 404s, not SPA
		// fallbacks — let the client see the real error.
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try the exact asset first.
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: rewrite to /index.html and let the file server
		// serve it (so the Vue router can resolve the route client-side).
		if f, err := sub.Open("index.html"); err == nil {
			f.Close()
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// dist/ has no index.html — frontend has not been built yet.
		c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusServiceUnavailable,
			`<!doctype html><html><body><h1>3xui-dashboard</h1>`+
				`<p>Frontend bundle is missing. Run <code>make frontend</code> or `+
				`<code>cd frontend &amp;&amp; npm run build</code> before serving.</p>`+
				`</body></html>`)
	})
}
