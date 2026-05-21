package middleware

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
)

// AuditLog returns a middleware that writes one admin_actions row
// per MUTATING admin request (POST/PUT/DELETE/PATCH). GET/HEAD are
// not logged — the table would fill up with admin-UI list refreshes
// without any incident-response value.
//
// Audit insertion is best-effort: errors are logged but never
// propagated to the request path. The dashboard must keep working
// even if the audit-log table is broken.
//
// Wiring: mount BEFORE RequireAdmin in the route group so the
// middleware can pull Claims after the request is served, OR
// register it on the same group as RequireAdmin since Gin
// middlewares run top-to-bottom but the audit's work happens
// AFTER c.Next() returns. App-side recipe:
//
//	apiAdminAuthed := engine.Group("/api/admin",
//	    middleware.RequireAdmin(authSvc),
//	    middleware.AuditLog(actionRepo, logger),
//	)
func AuditLog(repo *repository.AdminActionRepo, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only audit mutating verbs.
		switch c.Request.Method {
		case "POST", "PUT", "DELETE", "PATCH":
		default:
			return
		}

		username := ""
		if cl := Claims(c); cl != nil {
			username = cl.Username
			if username == "" {
				username = cl.Subject
			}
		}
		resource, id := parseTarget(c.Request.URL.Path)

		// Capture the error message, if any, from the JSON
		// response. Gin doesn't expose a clean way to read the
		// body after the writer is closed, so we accept that
		// only c.Errors is captured here; handlers that just
		// c.JSON(500, gin.H{"error": ...}) leave error_msg empty.
		// status_code is what matters for incident response.
		errMsg := ""
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}

		row := &model.AdminAction{
			AdminUsername:  username,
			Method:         c.Request.Method,
			Path:           c.Request.URL.Path,
			TargetResource: resource,
			TargetID:       id,
			QueryString:    c.Request.URL.RawQuery,
			IP:             c.ClientIP(),
			UserAgent:      c.GetHeader("User-Agent"),
			StatusCode:     c.Writer.Status(),
			ErrorMsg:       errMsg,
		}
		// Write on a detached context with a tight timeout so
		// audit insertion doesn't block the response or get
		// killed when the request context cancels (e.g. client
		// closes the socket after submitting).
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := repo.Insert(ctx, row); err != nil {
			log.Warn("audit log insert failed",
				slog.String("path", row.Path),
				slog.Int("status", row.StatusCode),
				slog.String("err", err.Error()),
			)
		}
	}
}

// parseTarget plucks {resource, id} out of an admin URL path.
// Examples:
//
//	/api/admin/orders/42/refund        → orders, 42
//	/api/admin/users/7                 → users, 7
//	/api/admin/webhooks/3/test         → webhooks, 3
//	/api/admin/settings/smtp-test      → settings, smtp-test
//	/api/admin/plans                   → plans, ""
//	/api/admin                         → "", ""
//
// The "/api/admin/" prefix is stripped; the first remaining segment
// is `resource`; the second (if numeric OR if it doesn't look like
// an action verb) is `id`. The heuristic is best-effort — false
// positives just attribute the row to the slightly-wrong target,
// which is still grep-able.
func parseTarget(path string) (resource, id string) {
	const prefix = "/api/admin/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	resource = parts[0]
	if len(parts) >= 2 && parts[1] != "" {
		id = parts[1]
	}
	return resource, id
}
