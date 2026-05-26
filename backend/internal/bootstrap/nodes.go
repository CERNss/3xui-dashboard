package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cern/3xui-dashboard/internal/config"
	nodesvc "github.com/cern/3xui-dashboard/internal/service/node"
)

type bootstrapNode struct {
	Name      string `json:"name"`
	Area      string `json:"area"`
	Province  string `json:"province"`
	Scheme    string `json:"scheme"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	BasePath  string `json:"base_path"`
	APIToken  string `json:"api_token"`
	Enabled   *bool  `json:"enabled"`
	AccessURL string `json:"access_url"`
}

// Nodes upserts the optional startup node list from configuration.
func Nodes(parent context.Context, cfg *config.Config, svc *nodesvc.Service, logger *slog.Logger) error {
	inputs, err := ParseNodes(cfg.Bootstrap.NodesJSON)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	for i, in := range inputs {
		node, created, err := svc.UpsertByName(ctx, in)
		if err != nil {
			return fmt.Errorf("bootstrap nodes[%d] upsert %q: %w", i, in.Name, err)
		}
		action := "updated"
		if created {
			action = "created"
		}
		logger.Info("bootstrap node "+action,
			slog.Int64("node_id", node.ID),
			slog.String("name", node.Name),
			slog.String("host", node.Host),
			slog.Int("port", node.Port),
			slog.Bool("enabled", node.Enabled),
		)
	}
	return nil
}

// ParseNodes turns a JSON env value into normalized node inputs.
func ParseNodes(raw string) ([]nodesvc.Input, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	raw = trimOuterEnvQuotes(raw)

	var rows []bootstrapNode
	if strings.HasPrefix(raw, "{") {
		var row bootstrapNode
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			return nil, fmt.Errorf("bootstrap nodes: decode JSON object: %w", err)
		}
		rows = append(rows, row)
	} else {
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return nil, fmt.Errorf("bootstrap nodes: decode JSON array: %w", err)
		}
	}

	out := make([]nodesvc.Input, 0, len(rows))
	for i, row := range rows {
		in, err := row.toInput()
		if err != nil {
			return nil, fmt.Errorf("bootstrap nodes[%d]: %w", i, err)
		}
		if err := nodesvc.Normalize(&in); err != nil {
			return nil, fmt.Errorf("bootstrap nodes[%d]: %w", i, err)
		}
		out = append(out, in)
	}
	return out, nil
}

func trimOuterEnvQuotes(raw string) string {
	if len(raw) < 2 {
		return raw
	}
	if raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

func (n bootstrapNode) toInput() (nodesvc.Input, error) {
	in := nodesvc.Input{
		Name:     n.Name,
		Area:     n.Area,
		Province: n.Province,
		Scheme:   n.Scheme,
		Host:     n.Host,
		Port:     n.Port,
		BasePath: n.BasePath,
		APIToken: n.APIToken,
		Enabled:  true,
	}
	if n.Enabled != nil {
		in.Enabled = *n.Enabled
	}
	if n.AccessURL != "" {
		if err := applyAccessURL(&in, n.AccessURL); err != nil {
			return in, err
		}
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = defaultBootstrapNodeName(in)
	}
	if strings.TrimSpace(in.Scheme) == "" {
		in.Scheme = "https"
	}
	return in, nil
}

func applyAccessURL(in *nodesvc.Input, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid access_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid access_url: scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid access_url: host is required")
	}
	port := 0
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("invalid access_url: bad port %q", p)
		}
		port = n
	} else if u.Scheme == "http" {
		port = 80
	} else {
		port = 443
	}

	in.Scheme = u.Scheme
	in.Host = host
	in.Port = port
	in.BasePath = panelPathFromURLPath(u.EscapedPath())
	return nil
}

func panelPathFromURLPath(pathname string) string {
	segments := strings.Split(strings.Trim(pathname, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		return "/panel/"
	}
	for i, item := range segments {
		if strings.EqualFold(item, "panel") {
			return "/" + strings.Join(segments[:i+1], "/") + "/"
		}
	}
	for i, item := range segments {
		if strings.EqualFold(item, "api") && i > 0 {
			return "/" + strings.Join(segments[:i], "/") + "/"
		}
	}
	return "/" + strings.Join(append(segments, "panel"), "/") + "/"
}

func defaultBootstrapNodeName(in nodesvc.Input) string {
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return ""
	}
	if in.Port > 0 {
		return fmt.Sprintf("%s:%d", host, in.Port)
	}
	return host
}
