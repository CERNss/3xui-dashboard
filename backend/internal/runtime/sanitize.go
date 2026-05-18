package runtime

import (
	"encoding/json"
	"fmt"
)

// sanitizeStreamSettingsForRemote scrubs filesystem cert paths from a
// stringified streamSettings JSON when inline cert content is also
// present. 3x-ui happily echoes back local file paths from the node
// side; if we pass them on to a different node those paths point at
// nothing and break TLS. Inline cert content (`certificate`/`key`
// arrays) is the portable form — strip the path fields when it's
// there.
//
// Returns the (possibly rewritten) string. On parse error the input
// is returned unchanged so a malformed payload does not block the
// update (the remote panel will surface its own error).
func sanitizeStreamSettingsForRemote(stream string) string {
	if stream == "" {
		return stream
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stream), &top); err != nil {
		return stream
	}

	// Possible cert-bearing keys depending on transport.
	for _, key := range []string{"tlsSettings", "xtlsSettings", "realitySettings", "tcpSettings", "wsSettings"} {
		raw, ok := top[key]
		if !ok || len(raw) == 0 {
			continue
		}
		rewritten, changed := stripCertPaths(raw)
		if changed {
			top[key] = rewritten
		}
	}

	out, err := json.Marshal(top)
	if err != nil {
		return stream
	}
	return string(out)
}

// stripCertPaths walks a TLS-settings sub-object and removes file-path
// fields from each certificates[] entry whose inline content is
// present.
func stripCertPaths(raw json.RawMessage) (json.RawMessage, bool) {
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		return raw, false
	}

	certsRaw, ok := settings["certificates"]
	if !ok {
		return raw, false
	}

	var certs []map[string]any
	if err := json.Unmarshal(certsRaw, &certs); err != nil {
		return raw, false
	}

	changed := false
	for i := range certs {
		c := certs[i]
		// If the inline content is present and non-empty, drop the
		// path field. We check both "certificate"/"key" and the
		// legacy "certificateFile"/"keyFile" form.
		if hasInlineContent(c, "certificate") {
			if _, ok := c["certificateFile"]; ok {
				delete(c, "certificateFile")
				changed = true
			}
		}
		if hasInlineContent(c, "key") {
			if _, ok := c["keyFile"]; ok {
				delete(c, "keyFile")
				changed = true
			}
		}
	}
	if !changed {
		return raw, false
	}
	settings["certificates"], _ = marshal(certs)
	out, err := marshal(settings)
	if err != nil {
		return raw, false
	}
	return out, true
}

func hasInlineContent(c map[string]any, key string) bool {
	v, ok := c[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	default:
		return false
	}
}

// marshal is a thin wrapper to keep the call sites compact.
func marshal(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("sanitize marshal: %w", err)
	}
	return b, nil
}
