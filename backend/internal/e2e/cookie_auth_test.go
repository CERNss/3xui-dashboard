package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"
)

// TestCookieAuthAndCSRF exercises the browser auth path end-to-end,
// which the Bearer-based harness helpers never touch:
//
//  1. login (no Authorization header) sets the httpOnly session cookie
//     and the readable CSRF cookie;
//  2. a cookie-only GET is authorized;
//  3. a cookie-only mutation is blocked (403) without the CSRF header;
//  4. the same mutation clears the CSRF gate once the header is echoed.
func TestCookieAuthAndCSRF(t *testing.T) {
	h := setupHarness(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}

	// 1. Login with credentials only — the browser never sends a Bearer
	//    header. The server should respond with Set-Cookie for both the
	//    session and the CSRF token.
	loginBody, _ := json.Marshal(map[string]string{"username": adminUser, "password": adminPass})
	resp, err := client.Post(h.URL("/api/admin/auth/login"), "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}

	var sessionSet, csrfToken string
	for _, c := range cookies {
		switch c.Name {
		case "3xui_admin_session":
			sessionSet = c.Value
		case "3xui_csrf":
			csrfToken = c.Value
		}
	}
	if sessionSet == "" {
		t.Fatal("login did not set the admin session cookie")
	}
	if csrfToken == "" {
		t.Fatal("login did not set the CSRF cookie")
	}

	// 2. Cookie-only authed GET — the jar replays the session cookie; no
	//    Authorization header is set. Expect 200.
	getResp, err := client.Get(h.URL("/api/admin/nodes"))
	if err != nil {
		t.Fatalf("authed GET: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("cookie-authed GET status = %d, want 200", getResp.StatusCode)
	}

	// 3. Cookie-only mutation WITHOUT the CSRF header — blocked at 403
	//    by the CSRF guard before the handler ever runs.
	noCSRF, err := client.Post(h.URL("/api/admin/nodes"), "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("mutation without CSRF: %v", err)
	}
	noCSRF.Body.Close()
	if noCSRF.StatusCode != http.StatusForbidden {
		t.Fatalf("cookie mutation without CSRF = %d, want 403", noCSRF.StatusCode)
	}

	// 4. Same mutation WITH the CSRF header echoed — clears the gate.
	//    The handler then rejects the empty body (400); the point is the
	//    request is no longer 403.
	req, _ := http.NewRequest(http.MethodPost, h.URL("/api/admin/nodes"), bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	withCSRF, err := client.Do(req)
	if err != nil {
		t.Fatalf("mutation with CSRF: %v", err)
	}
	withCSRF.Body.Close()
	if withCSRF.StatusCode == http.StatusForbidden {
		t.Fatal("cookie mutation with a matching CSRF token should not be 403")
	}
}
