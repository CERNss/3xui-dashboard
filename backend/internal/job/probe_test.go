package job

import (
	"errors"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// transitionCase is one row of the probe transition truth table.
// `wantEvents` is the ordered list of event types expected; payload
// content is checked separately to keep the table compact.
type transitionCase struct {
	desc       string
	prior      string
	probeErr   error
	wantEvents []string
}

func TestProbeTransitionEvents(t *testing.T) {
	probeErr := errors.New("connection refused")

	cases := []transitionCase{
		{
			desc:       "steady online: no events",
			prior:      model.NodeStatusOnline,
			probeErr:   nil,
			wantEvents: nil,
		},
		{
			desc:       "online → fail: probe_failed + offline",
			prior:      model.NodeStatusOnline,
			probeErr:   probeErr,
			wantEvents: []string{event.NodeProbeFailed, event.NodeOffline},
		},
		{
			desc:       "offline → ok: online + recovered (the headline scenario)",
			prior:      model.NodeStatusOffline,
			probeErr:   nil,
			wantEvents: []string{event.NodeOnline, event.NodeRecovered},
		},
		{
			desc:       "offline → fail: just probe_failed (not duplicate offline)",
			prior:      model.NodeStatusOffline,
			probeErr:   probeErr,
			wantEvents: []string{event.NodeProbeFailed},
		},
		{
			desc:       "unknown → ok: online only, NOT recovered (first probe)",
			prior:      model.NodeStatusUnknown,
			probeErr:   nil,
			wantEvents: []string{event.NodeOnline},
		},
		{
			desc:       "unknown → fail: just probe_failed",
			prior:      model.NodeStatusUnknown,
			probeErr:   probeErr,
			wantEvents: []string{event.NodeProbeFailed},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := probeTransitionEvents(42, "tokyo-1", c.prior, c.probeErr)
			if len(got) != len(c.wantEvents) {
				t.Fatalf("event count = %d (%v), want %d (%v)",
					len(got), eventTypes(got), len(c.wantEvents), c.wantEvents)
			}
			for i, want := range c.wantEvents {
				if got[i].Type != want {
					t.Errorf("event[%d].Type = %q, want %q", i, got[i].Type, want)
				}
			}
		})
	}
}

// TestProbeTransitionEvents_RecoveryDistinction is the specific
// regression test for #7: NodeRecovered SHOULD fire on offline→online
// but NOT on unknown→online (startup case). If someone "simplifies"
// probe.go and conflates these, this test catches it.
func TestProbeTransitionEvents_RecoveryDistinction(t *testing.T) {
	fromOffline := probeTransitionEvents(1, "x", model.NodeStatusOffline, nil)
	fromUnknown := probeTransitionEvents(1, "x", model.NodeStatusUnknown, nil)

	hasRecovered := func(evs []probeTransitionEvent) bool {
		for _, e := range evs {
			if e.Type == event.NodeRecovered {
				return true
			}
		}
		return false
	}
	if !hasRecovered(fromOffline) {
		t.Error("offline → online should produce NodeRecovered")
	}
	if hasRecovered(fromUnknown) {
		t.Error("unknown → online MUST NOT produce NodeRecovered (would spam ops on boot)")
	}
}

// TestProbeTransitionEvents_PayloadShape verifies the payload struct
// is populated correctly — fields the notify service relies on for
// rendering messages.
func TestProbeTransitionEvents_PayloadShape(t *testing.T) {
	got := probeTransitionEvents(42, "tokyo-1", model.NodeStatusOffline, nil)
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	// First = NodeOnline
	if p, ok := got[0].Payload.(NodeStatusChangedPayload); !ok {
		t.Errorf("event 0 payload type = %T", got[0].Payload)
	} else {
		if p.NodeID != 42 || p.Name != "tokyo-1" || p.Now != model.NodeStatusOnline {
			t.Errorf("event 0 payload wrong: %+v", p)
		}
	}
	// Second = NodeRecovered
	if p, ok := got[1].Payload.(NodeStatusChangedPayload); !ok {
		t.Errorf("event 1 payload type = %T", got[1].Payload)
	} else {
		if p.Prior != model.NodeStatusOffline {
			t.Errorf("recovered.Prior = %q, want offline", p.Prior)
		}
	}
}

// TestProbeTransitionEvents_FailurePayloadCarriesError verifies the
// probe error message reaches subscribers (notify telegram channel
// uses it to format the alert body).
func TestProbeTransitionEvents_FailurePayloadCarriesError(t *testing.T) {
	got := probeTransitionEvents(1, "x", model.NodeStatusOnline, errors.New("dial tcp: i/o timeout"))
	if len(got) == 0 {
		t.Fatal("expected events")
	}
	p, ok := got[0].Payload.(NodeProbeFailedPayload)
	if !ok {
		t.Fatalf("event 0 payload = %T, want NodeProbeFailedPayload", got[0].Payload)
	}
	if p.Error != "dial tcp: i/o timeout" {
		t.Errorf("Error = %q", p.Error)
	}
}

func eventTypes(evs []probeTransitionEvent) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Type
	}
	return out
}
