package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Envelope is the 3x-ui `entity.Msg` wrapper around every panel
// response. Success is the panel's view of the call's outcome; Msg
// carries a human-readable error or status string; Obj is the
// per-endpoint payload (possibly null / absent).
type Envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

// EnvelopeError is the error returned when the panel reports
// success=false. Callers can errors.As into it to recover the panel
// message and any payload.
type EnvelopeError struct {
	Msg     string
	Payload json.RawMessage
	Path    string // request path that produced this error, for context
}

func (e *EnvelopeError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("3xui %s: %s", e.Path, e.Msg)
	}
	return "3xui: " + e.Msg
}

// DecodeObj decodes env.Obj into out. Returns ErrEmptyObj if there is
// nothing to decode (panel returned null or omitted the field) — most
// endpoints with mutate-only semantics return success/msg without an
// obj payload.
func (env *Envelope) DecodeObj(out any) error {
	if len(env.Obj) == 0 {
		return ErrEmptyObj
	}
	s := string(env.Obj)
	if s == "null" {
		return ErrEmptyObj
	}
	if err := json.Unmarshal(env.Obj, out); err != nil {
		return fmt.Errorf("decode envelope obj: %w", err)
	}
	return nil
}

// ErrEmptyObj is returned by Envelope.DecodeObj when the payload is
// absent or explicit null.
var ErrEmptyObj = errors.New("3xui: empty obj payload")
