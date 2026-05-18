package client

import (
	"encoding/json"
	"fmt"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// parseClientsFromInbound extracts the clients slice from the
// stringified-JSON settings field on an Inbound.
func parseClientsFromInbound(in *runtime.Inbound) ([]runtime.Client, error) {
	if in == nil || in.Settings == "" {
		return nil, nil
	}
	var s runtime.InboundSettings
	if err := json.Unmarshal([]byte(in.Settings), &s); err != nil {
		return nil, fmt.Errorf("parse inbound settings: %w", err)
	}
	return s.Clients, nil
}
