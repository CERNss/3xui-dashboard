package xui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cern/3xui-dashboard/config"
)

// Client is an HTTP client for the 3x-ui panel API.
type Client struct {
	baseURL  string
	basePath string
	token    string
	http     *http.Client
}

// NewClient builds a Client from the global config.
func NewClient() *Client {
	return &Client{
		baseURL:  config.C.XUI.BaseURL,
		basePath: config.C.XUI.BasePath,
		token:    config.C.XUI.APIToken,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) url(path string) string {
	return c.baseURL + c.basePath + path
}

// apiResponse is the generic 3x-ui API response wrapper.
type apiResponse struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

func (c *Client) do(method, path string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.url(path), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		// Non-JSON response (e.g. HTML error page)
		return nil, fmt.Errorf("parse 3x-ui response (status %d): %s", resp.StatusCode, string(raw))
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("3x-ui API error: %s", apiResp.Msg)
	}

	return apiResp.Obj, nil
}

// ---- Inbounds ----

// ListInbounds returns the raw JSON array of inbounds.
func (c *Client) ListInbounds() (json.RawMessage, error) {
	return c.do(http.MethodGet, "/panel/api/inbounds/list", nil)
}

// AddInbound creates a new inbound.
func (c *Client) AddInbound(payload interface{}) (json.RawMessage, error) {
	return c.do(http.MethodPost, "/panel/api/inbounds/add", payload)
}

// UpdateInbound updates an existing inbound by ID.
func (c *Client) UpdateInbound(id int, payload interface{}) (json.RawMessage, error) {
	return c.do(http.MethodPost, fmt.Sprintf("/panel/api/inbounds/update/%d", id), payload)
}

// DeleteInbound deletes an inbound by ID.
func (c *Client) DeleteInbound(id int) (json.RawMessage, error) {
	return c.do(http.MethodPost, fmt.Sprintf("/panel/api/inbounds/del/%d", id), nil)
}

// ---- Clients ----

// AddClient adds a client to an inbound.
func (c *Client) AddClient(payload interface{}) (json.RawMessage, error) {
	return c.do(http.MethodPost, "/panel/api/inbounds/addClient", payload)
}

// UpdateClient updates a client by UUID.
func (c *Client) UpdateClient(uuid string, payload interface{}) (json.RawMessage, error) {
	return c.do(http.MethodPost, fmt.Sprintf("/panel/api/inbounds/updateClient/%s", uuid), payload)
}

// DeleteClient deletes a client by UUID.
func (c *Client) DeleteClient(uuid string) (json.RawMessage, error) {
	return c.do(http.MethodPost, fmt.Sprintf("/panel/api/inbounds/delClient/%s", uuid), nil)
}

// GetClientTraffics returns traffic stats for a client identified by email.
func (c *Client) GetClientTraffics(email string) (json.RawMessage, error) {
	return c.do(http.MethodGet, fmt.Sprintf("/panel/api/inbounds/getClientTraffics/%s", email), nil)
}

// ---- Nodes ----

// ListNodes returns the raw JSON array of nodes.
func (c *Client) ListNodes() (json.RawMessage, error) {
	return c.do(http.MethodGet, "/panel/api/nodes/", nil)
}

// ---- Server ----

// ServerStatus returns the server status object.
func (c *Client) ServerStatus() (json.RawMessage, error) {
	return c.do(http.MethodGet, "/panel/api/server/status", nil)
}
