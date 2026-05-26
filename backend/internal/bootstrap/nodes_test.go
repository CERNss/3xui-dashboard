package bootstrap

import "testing"

func TestParseBootstrapNodes_AccessURLObject(t *testing.T) {
	raw := `{"name":"BWG US Node","area":"us","province":"California","access_url":"https://node.example.com:10138/z5obK8SjreIGDvfTvD","api_token":"tok","enabled":true}`

	nodes, err := ParseNodes(raw)
	if err != nil {
		t.Fatalf("parseBootstrapNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	got := nodes[0]
	if got.Name != "BWG US Node" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Scheme != "https" {
		t.Errorf("Scheme = %q", got.Scheme)
	}
	if got.Host != "node.example.com" {
		t.Errorf("Host = %q", got.Host)
	}
	if got.Port != 10138 {
		t.Errorf("Port = %d", got.Port)
	}
	if got.BasePath != "/z5obK8SjreIGDvfTvD/panel/" {
		t.Errorf("BasePath = %q", got.BasePath)
	}
	if got.APIToken != "tok" {
		t.Errorf("APIToken = %q", got.APIToken)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestParseBootstrapNodes_ArrayAndDefaultEnabled(t *testing.T) {
	raw := `[{"name":"Disabled","scheme":"http","host":"disabled.example.com","port":8080,"api_token":"tok","enabled":false},{"access_url":"https://node.example.com/panel","api_token":"tok"}]`

	nodes, err := ParseNodes(raw)
	if err != nil {
		t.Fatalf("parseBootstrapNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(nodes))
	}
	if nodes[0].Enabled {
		t.Error("first node Enabled = true, want false")
	}
	if nodes[1].Name != "node.example.com:443" {
		t.Errorf("default name = %q", nodes[1].Name)
	}
	if !nodes[1].Enabled {
		t.Error("second node Enabled = false, want default true")
	}
}

func TestParseBootstrapNodes_RejectsBadAccessURL(t *testing.T) {
	_, err := ParseNodes(`{"access_url":"ftp://node.example.com/panel","api_token":"tok"}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPanelPathFromURLPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "/panel/"},
		{"/", "/panel/"},
		{"/panel", "/panel/"},
		{"/root/panel/inbounds", "/root/panel/"},
		{"/root/api/inbounds/list", "/root/"},
		{"/z5obK8SjreIGDvfTvD", "/z5obK8SjreIGDvfTvD/panel/"},
	}
	for _, tc := range cases {
		if got := panelPathFromURLPath(tc.in); got != tc.want {
			t.Errorf("panelPathFromURLPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
