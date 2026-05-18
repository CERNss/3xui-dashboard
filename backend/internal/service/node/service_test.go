package node

import (
	"errors"
	"testing"
)

func TestNormalize_TrimsAndLowercases(t *testing.T) {
	in := Input{
		Name:     "  Node One ",
		Scheme:   " HTTPS ",
		Host:     " host.example.com ",
		Port:     2053,
		BasePath: "panel",
		APIToken: " token ",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Name != "Node One" {
		t.Errorf("Name = %q, want trimmed", in.Name)
	}
	if in.Scheme != "https" {
		t.Errorf("Scheme = %q, want https", in.Scheme)
	}
	if in.BasePath != "/panel/" {
		t.Errorf("BasePath = %q, want /panel/", in.BasePath)
	}
	if in.APIToken != "token" {
		t.Errorf("APIToken = %q, want trimmed", in.APIToken)
	}
}

func TestNormalize_RejectsBadInput(t *testing.T) {
	cases := []struct {
		desc string
		in   Input
	}{
		{"empty name", Input{Scheme: "https", Host: "h", Port: 1}},
		{"bad scheme", Input{Name: "x", Scheme: "ftp", Host: "h", Port: 1}},
		{"empty host", Input{Name: "x", Scheme: "https", Port: 1}},
		{"zero port", Input{Name: "x", Scheme: "https", Host: "h"}},
		{"negative port", Input{Name: "x", Scheme: "https", Host: "h", Port: -5}},
		{"oversize port", Input{Name: "x", Scheme: "https", Host: "h", Port: 70000}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if err := Normalize(&c.in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"/", ""},
		{"panel", "/panel/"},
		{"/panel", "/panel/"},
		{"panel/", "/panel/"},
		{"/panel/", "/panel/"},
	}
	for _, c := range cases {
		if got := normalizeBasePath(c.in); got != c.want {
			t.Errorf("normalizeBasePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
