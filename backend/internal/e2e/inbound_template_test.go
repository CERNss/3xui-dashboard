package e2e

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

// e2e coverage for POST /api/admin/inbounds/nodes/:nodeID with the
// optional `template_id` sidecar field. Four branches:
//
//  1. happy path — body has template_id + port + tag, server
//     materializes from template via BuildTemplateInbound and POSTs
//     to the upstream mock panel.
//  2. template_id not in DB → 404.
//  3. template_id resolves but template.enabled == false → 400.
//  4. template_id set but body.port missing/zero → 400.
//
// All four exercise the real wiring at /api/admin/inbounds + the
// template lookup path bridged via repository.ProvisioningPoolRepo.

func seedNodeForTemplateTest(t *testing.T, h *harness, adminTok string) int64 {
	t.Helper()
	u, _ := url.Parse(h.panel.URL())
	port, _ := strconv.Atoi(u.Port())
	var node struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/nodes",
		token:  adminTok,
		body: map[string]any{
			"name": "tpl-node", "scheme": "http", "host": u.Hostname(),
			"port": port, "api_token": "x", "enabled": true,
		},
	}, &node); got != http.StatusCreated {
		t.Fatalf("create node: status=%d", got)
	}
	return node.ID
}

func createTemplate(t *testing.T, h *harness, adminTok string, name string, enabled bool) int64 {
	t.Helper()
	var tmpl struct {
		ID int64 `json:"id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/inbound-templates",
		token:  adminTok,
		body: map[string]any{
			"name": name, "enabled": enabled, "protocol": "vless",
			"remark": name, "listen": "",
			"settings":       `{"clients":[],"decryption":"none"}`,
			"streamSettings": `{"network":"tcp","security":"none"}`,
			"sniffing":       `{}`,
			"trafficReset":   "never",
			"total":          0,
			"expiryTime":     0,
		},
	}, &tmpl); got != http.StatusCreated {
		t.Fatalf("create template %q: status=%d", name, got)
	}
	return tmpl.ID
}

func TestInboundCreate_FromTemplate_HappyPath(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)
	nodeID := seedNodeForTemplateTest(t, h, adminTok)
	templateID := createTemplate(t, h, adminTok, "vless-tcp-plain", true)

	var created struct {
		ID             int64  `json:"id"`
		Tag            string `json:"tag"`
		Protocol       string `json:"protocol"`
		Port           int    `json:"port"`
		Settings       string `json:"settings"`
		StreamSettings string `json:"streamSettings"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/inbounds/nodes/" + itoa(nodeID),
		token:  adminTok,
		body: map[string]any{
			"tag":         "vless-18081",
			"port":        18081,
			"template_id": templateID,
			"remark":      "from-template",
			"enable":      true,
		},
	}, &created); got != http.StatusCreated {
		t.Fatalf("create from template: status=%d", got)
	}

	if created.Tag != "vless-18081" {
		t.Errorf("tag = %q, want vless-18081", created.Tag)
	}
	if created.Protocol != "vless" {
		t.Errorf("protocol = %q, want vless (template wins)", created.Protocol)
	}
	if created.Port != 18081 {
		t.Errorf("port = %d, want 18081", created.Port)
	}
	if created.StreamSettings != `{"network":"tcp","security":"none"}` {
		t.Errorf("streamSettings = %q, want template's", created.StreamSettings)
	}

	// Mock panel saw the create.
	if h.panel.Calls("/panel/api/inbounds/add") != 1 {
		t.Errorf("mock panel /add hit %d times, want 1", h.panel.Calls("/panel/api/inbounds/add"))
	}
}

func TestInboundCreate_FromTemplate_NotFound(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)
	nodeID := seedNodeForTemplateTest(t, h, adminTok)

	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/inbounds/nodes/" + itoa(nodeID),
		token:  adminTok,
		body: map[string]any{
			"tag":         "vless-18082",
			"port":        18082,
			"template_id": 99999,
		},
	}, nil); got != http.StatusNotFound {
		t.Fatalf("non-existent template should 404; got %d", got)
	}
}

func TestInboundCreate_FromTemplate_Disabled(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)
	nodeID := seedNodeForTemplateTest(t, h, adminTok)
	templateID := createTemplate(t, h, adminTok, "vless-disabled", false)

	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/inbounds/nodes/" + itoa(nodeID),
		token:  adminTok,
		body: map[string]any{
			"tag":         "vless-18083",
			"port":        18083,
			"template_id": templateID,
		},
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("disabled template should 400; got %d", got)
	}
}

func TestInboundCreate_FromTemplate_MissingPort(t *testing.T) {
	h := setupHarness(t)
	adminTok := h.adminLogin(t)
	nodeID := seedNodeForTemplateTest(t, h, adminTok)
	templateID := createTemplate(t, h, adminTok, "vless-no-port", true)

	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/inbounds/nodes/" + itoa(nodeID),
		token:  adminTok,
		body: map[string]any{
			"tag":         "vless-noport",
			"template_id": templateID,
			// port intentionally missing
		},
	}, nil); got != http.StatusBadRequest {
		t.Fatalf("missing port should 400; got %d", got)
	}
}
