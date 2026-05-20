// Package channels contains concrete notify.Channel implementations.
// Each file is one provider — see telegram.go, discord.go, feishu.go.
// httputil.go is the only shared helper: a tiny retry+rate-limit wrapper
// for one-shot webhook POSTs that all 3 IM channels need.
package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// PostJSONOptions tunes one-off POST retry behavior. Zero values
// give: 1 retry, 1s base delay, 30s Retry-After cap.
type PostJSONOptions struct {
	MaxRetries     int
	BaseRetryDelay time.Duration
	RetryAfterCap  time.Duration
}

// PostJSON sends `payload` as JSON to `url`. On HTTP 5xx / network
// error it retries once after BaseRetryDelay; on HTTP 429 it honors
// Retry-After up to RetryAfterCap; on HTTP 4xx it returns the error
// immediately (no retry — that's a config issue, not transient).
//
// The function reads up to 1 KiB of the response body so the caller
// can surface the server's error message in logs. Beyond 1 KiB
// gets discarded to bound memory under spammy 4xx loops.
func PostJSON(ctx context.Context, client *http.Client, url string, payload any, opts PostJSONOptions) (statusCode int, body []byte, err error) {
	maxRetries := opts.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries == 0 {
		maxRetries = 1
	}
	baseDelay := opts.BaseRetryDelay
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	cap429 := opts.RetryAfterCap
	if cap429 <= 0 {
		cap429 = 30 * time.Second
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			case <-time.After(baseDelay):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return 0, nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http: %w", err)
			continue
		}
		statusCode = resp.StatusCode
		body, _ = io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()

		switch {
		case statusCode >= 200 && statusCode < 300:
			return statusCode, body, nil
		case statusCode == http.StatusTooManyRequests:
			// Honor Retry-After up to cap; if attempts exhausted,
			// return so the caller can log + give up.
			if attempt == maxRetries {
				return statusCode, body, fmt.Errorf("rate limited (HTTP 429): %s", truncate(string(body), 200))
			}
			d := parseRetryAfter(resp.Header.Get("Retry-After"), cap429)
			select {
			case <-ctx.Done():
				return statusCode, body, ctx.Err()
			case <-time.After(d):
			}
		case statusCode >= 400 && statusCode < 500:
			// Config error — don't retry.
			return statusCode, body, fmt.Errorf("HTTP %d: %s", statusCode, truncate(string(body), 200))
		default:
			// 5xx — fall through to retry loop.
			lastErr = fmt.Errorf("HTTP %d: %s", statusCode, truncate(string(body), 200))
		}
	}
	if lastErr != nil {
		return statusCode, body, lastErr
	}
	return statusCode, body, fmt.Errorf("retries exhausted")
}

func parseRetryAfter(header string, cap time.Duration) time.Duration {
	if header == "" {
		return 1 * time.Second
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		d := time.Duration(seconds) * time.Second
		if d > cap {
			return cap
		}
		return d
	}
	// HTTP-date format is allowed by spec but rare in API responses;
	// fall back to a small fixed wait.
	return 1 * time.Second
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
