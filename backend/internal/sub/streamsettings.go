package sub

import (
	"encoding/base64"
	"encoding/json"
)

// parseStreamSettings returns a generic map from the stringified-JSON
// streamSettings field. Empty input or unparsable JSON yields nil.
func parseStreamSettings(stream string) map[string]any {
	if stream == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stream), &out); err != nil {
		return nil
	}
	return out
}

func ssObj(m map[string]any, key string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	sub, ok := v.(map[string]any)
	return sub, ok
}

func stringFrom(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			if t == "" {
				return fallback
			}
			return t
		case bool:
			if t {
				return "true"
			}
			return "false"
		}
	}
	return fallback
}

func stringSliceFrom(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	if s, ok := v.([]any); ok {
		out := make([]string, 0, len(s))
		for _, x := range s {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func firstString(m map[string]any, key string) string {
	s := stringSliceFrom(m, key)
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// wsHost reads the Host header from wsSettings — newer 3x-ui keeps
// it under `headers.Host`, older versions stored it at `host` (string).
func wsHost(ws map[string]any) string {
	if h := stringFrom(ws, "host", ""); h != "" {
		return h
	}
	if hdrs, ok := ssObj(ws, "headers"); ok {
		if v := stringFrom(hdrs, "Host", ""); v != "" {
			return v
		}
		if v := stringFrom(hdrs, "host", ""); v != "" {
			return v
		}
	}
	return ""
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// base64URLNoPad returns the unpadded URL-safe base64 of b. Most
// client apps accept either standard or URL-safe base64; using URL-
// safe avoids '/' and '+' which need escaping in the link.
func base64URLNoPad(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
