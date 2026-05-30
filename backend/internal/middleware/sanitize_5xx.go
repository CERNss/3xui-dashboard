package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Sanitize5xx wraps the response writer so that 5xx responses with a
// JSON body of shape {"error": "..."} have the error string replaced
// by a generic message before being sent to the client. The original
// detail is logged at WARN level for operator debugging.
//
// Intended to gate user-facing endpoints (/api/user/*, /sub/*) where
// internal error detail (database hostnames, file paths, panel URLs)
// would otherwise leak to unauthenticated or low-trust callers. Admin
// endpoints keep the raw detail because admins need it.
//
// Note: this captures the entire body in memory. That's fine for
// error responses (small JSON) — never mount on streaming endpoints.
func Sanitize5xx(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		bw := &bufferedResponseWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = bw
		c.Next()

		status := bw.Status()
		if status < 500 || status > 599 {
			// Non-5xx — forward whatever the handler produced unchanged.
			_, _ = bw.ResponseWriter.Write(bw.buf.Bytes())
			return
		}

		// Try to recognise a {"error": "..."} body and replace it.
		body := bw.buf.Bytes()
		var parsed map[string]any
		if json.Unmarshal(body, &parsed) == nil {
			if original, ok := parsed["error"].(string); ok && original != "" {
				log.Warn("sanitized 5xx response detail",
					slog.Int("status", status),
					slog.String("path", c.Request.URL.Path),
					slog.String("original_error", original),
				)
				parsed["error"] = "internal server error"
				if newBody, err := json.Marshal(parsed); err == nil {
					_, _ = bw.ResponseWriter.Write(newBody)
					return
				}
			}
		}

		// Fall back to forwarding the body verbatim if we couldn't
		// parse it. Avoids dropping non-JSON 5xx bodies entirely.
		_, _ = bw.ResponseWriter.Write(body)
	}
}

type bufferedResponseWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bufferedResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *bufferedResponseWriter) WriteString(s string) (int, error) {
	return w.buf.WriteString(s)
}

// Make sure we still report Hijack/Flush/CloseNotify correctly when
// the wrapper is used with HTTP/1.1 features. The embedded
// gin.ResponseWriter passes those through.
var _ http.ResponseWriter = (*bufferedResponseWriter)(nil)
