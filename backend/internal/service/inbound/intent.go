package inbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

// resolveIntent reads the `_intent` sub-block (if any) on the inbound's
// settings + streamSettings JSON strings, calls the matching panel
// endpoints on the target node to materialize concrete keys / certs,
// substitutes them into the inbound, and strips the intent block so
// the wire payload xray-core actually receives is plain xray JSON
// with no dashboard-specific markers.
//
// The dashboard's template editor expresses Reality keypair generation,
// ML-DSA-65 seed generation, VLESS encryption mode (x25519 / mlkem768),
// and short-IDs randomization as intent flags. This is the single
// place those flags become real values.
//
// The function mutates `in` in place. If no _intent block is present
// it is a no-op.
func resolveIntent(ctx context.Context, r *runtime.Remote, in *runtime.Inbound) error {
	if in == nil || r == nil {
		return nil
	}
	if err := resolveSettingsIntent(ctx, r, in); err != nil {
		return err
	}
	if err := resolveStreamIntent(ctx, r, in); err != nil {
		return err
	}
	return nil
}

func resolveSettingsIntent(ctx context.Context, r *runtime.Remote, in *runtime.Inbound) error {
	settings, ok := decodeIntentObject(in.Settings)
	if !ok {
		return nil
	}
	intent, hasIntent := settings["_intent"].(map[string]any)
	if !hasIntent {
		return nil
	}

	if vlessAuth, _ := intent["vlessAuth"].(string); vlessAuth == "x25519" || vlessAuth == "mlkem768" {
		resp, err := r.GetNewVlessEnc(ctx)
		if err != nil {
			return fmt.Errorf("getNewVlessEnc: %w", err)
		}
		var picked *runtime.VlessEncAuth
		for i := range resp.Auths {
			if resp.Auths[i].ID == vlessAuth {
				picked = &resp.Auths[i]
				break
			}
		}
		if picked == nil {
			return fmt.Errorf("vlessenc: no auth with id %q in panel response", vlessAuth)
		}
		settings["decryption"] = picked.Decryption
		settings["encryption"] = picked.Encryption
	}

	if asBool(intent["wireguardKeypair"]) {
		kp, err := wgcrypto.GenerateKeypair()
		if err != nil {
			return fmt.Errorf("generate WG keypair: %w", err)
		}
		settings["secretKey"] = kp.Private
		settings["pubKey"] = kp.Public
	}

	delete(settings, "_intent")
	encoded, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("re-marshal settings: %w", err)
	}
	in.Settings = string(encoded)
	return nil
}

func resolveStreamIntent(ctx context.Context, r *runtime.Remote, in *runtime.Inbound) error {
	stream, ok := decodeIntentObject(in.StreamSettings)
	if !ok {
		return nil
	}
	intent, hasIntent := stream["_intent"].(map[string]any)
	if !hasIntent {
		return nil
	}

	reality, _ := stream["realitySettings"].(map[string]any)
	if reality == nil {
		reality = make(map[string]any)
		stream["realitySettings"] = reality
	}

	if asBool(intent["realityKeypair"]) {
		cert, err := r.GetNewX25519Cert(ctx)
		if err != nil {
			return fmt.Errorf("getNewX25519Cert: %w", err)
		}
		reality["privateKey"] = cert.PrivateKey
		reality["publicKey"] = cert.PublicKey
	}

	if asBool(intent["realityMldsa65"]) {
		cert, err := r.GetNewMldsa65(ctx)
		if err != nil {
			return fmt.Errorf("getNewMldsa65: %w", err)
		}
		reality["mldsa65Seed"] = cert.Seed
		reality["mldsa65Verify"] = cert.Verify
	}

	// Local-only randomizers (no panel round-trip).
	if asBool(intent["realityRandomShortIds"]) {
		ids, err := randomShortIDs(8)
		if err != nil {
			return fmt.Errorf("random short IDs: %w", err)
		}
		reality["shortIds"] = ids
	}
	// realityRandomTarget / realityRandomSNI: not implemented yet because
	// picking a "good" Reality destination requires a curated pool of
	// CDN endpoints that pass Reality probes; for now those flags get
	// stripped without effect so the operator's typed-in values survive.

	delete(stream, "_intent")
	encoded, err := json.Marshal(stream)
	if err != nil {
		return fmt.Errorf("re-marshal streamSettings: %w", err)
	}
	in.StreamSettings = string(encoded)
	return nil
}

func decodeIntentObject(s string) (map[string]any, bool) {
	if s == "" {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, false
	}
	return out, true
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func randomShortIDs(count int) ([]string, error) {
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// Reality short IDs are 1..8 byte hex strings. We emit a
		// distribution that mirrors the upstream panel: one 1-byte,
		// then progressively longer up to 8.
		length := i + 1
		if length > 8 {
			length = 8
		}
		buf := make([]byte, length)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		ids = append(ids, hex.EncodeToString(buf))
	}
	return ids, nil
}
