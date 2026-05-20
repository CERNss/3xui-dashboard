package sub

// sip008Server is one entry in a SIP008 subscription document.
// Spec: https://shadowsocks.org/doc/sip008.html
type sip008Server struct {
	ID         string `json:"id"`
	Remarks    string `json:"remarks"`
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
	Password   string `json:"password"`
	Method     string `json:"method"`
}

// sip008Doc is the top-level SIP008 v1 wrapper.
type sip008Doc struct {
	Version  int            `json:"version"`
	Username string         `json:"username,omitempty"`
	Servers  []sip008Server `json:"servers"`
}

// buildSIP008 filters d.Links to Shadowsocks clients and emits the
// SIP008 document. Non-SS links are silently dropped — SIP008 is an
// SS-specific format by design.
func buildSIP008(d *SubscriptionData) sip008Doc {
	doc := sip008Doc{Version: 1, Servers: []sip008Server{}}
	if d == nil {
		return doc
	}
	if d.User != nil && d.User.SubID != "" {
		doc.Username = d.User.SubID
	}
	for _, l := range d.Links {
		if l.Protocol != "shadowsocks" {
			continue
		}
		method, password := parseShadowsocksAuth(l.Inbound, l.Client)
		doc.Servers = append(doc.Servers, sip008Server{
			ID:         l.Client.ID,
			Remarks:    l.Remark,
			Server:     l.Host,
			ServerPort: l.Port,
			Password:   password,
			Method:     method,
		})
	}
	return doc
}
