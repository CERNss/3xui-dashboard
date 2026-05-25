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
	if in.Area != "unknown" {
		t.Errorf("Area = %q, want unknown default", in.Area)
	}
	if in.Province != "unknown" {
		t.Errorf("Province = %q, want unknown default", in.Province)
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

func TestNormalize_TrimsAreaAndProvince(t *testing.T) {
	in := Input{
		Name:     "n",
		Area:     " sg ",
		Province: "  Central  ",
		Scheme:   "https",
		Host:     "host.example.com",
		Port:     2053,
		APIToken: "token",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Area != "sg" {
		t.Errorf("Area = %q, want sg", in.Area)
	}
	if in.Province != "Central" {
		t.Errorf("Province = %q, want Central", in.Province)
	}
}

func TestNormalize_DetectsAreaAndProvince(t *testing.T) {
	in := Input{
		Name:     "us-redmond-1",
		Scheme:   "https",
		Host:     "panel.seattle.example.com",
		Port:     2053,
		APIToken: "token",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Area != "us" {
		t.Errorf("Area = %q, want us", in.Area)
	}
	if in.Province != "Washington" {
		t.Errorf("Province = %q, want Washington", in.Province)
	}
}

func TestNormalize_DetectsAreaAndFallsBackUnknownProvince(t *testing.T) {
	in := Input{
		Name:     "singapore-edge",
		Scheme:   "https",
		Host:     "panel.example.com",
		Port:     2053,
		APIToken: "token",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Area != "sg" {
		t.Errorf("Area = %q, want sg", in.Area)
	}
	if in.Province != "unknown" {
		t.Errorf("Province = %q, want unknown", in.Province)
	}
}

func TestNormalize_DetectsAreaFromProvince(t *testing.T) {
	in := Input{
		Name:     "edge",
		Province: "Tokyo",
		Scheme:   "https",
		Host:     "host.example.com",
		Port:     2053,
		APIToken: "token",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Area != "jp" {
		t.Errorf("Area = %q, want jp", in.Area)
	}
	if in.Province != "Tokyo" {
		t.Errorf("Province = %q, want Tokyo", in.Province)
	}
}

func TestNormalize_UnknownAreaStillAllowsInput(t *testing.T) {
	in := Input{
		Name:     "x",
		Area:     "mars",
		Scheme:   "https",
		Host:     "host.example.com",
		Port:     2053,
		APIToken: "token",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Area != "unknown" {
		t.Errorf("Area = %q, want unknown", in.Area)
	}
	if in.Province != "unknown" {
		t.Errorf("Province = %q, want unknown", in.Province)
	}
}

func TestNormalize_UnknownProvinceTriggersDetection(t *testing.T) {
	in := Input{
		Name:     "redmond-panel",
		Area:     "us",
		Province: "未知",
		Scheme:   "https",
		Host:     "host.example.com",
		Port:     2053,
		APIToken: "token",
	}
	if err := Normalize(&in); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if in.Area != "us" {
		t.Errorf("Area = %q, want us", in.Area)
	}
	if in.Province != "Redmond" {
		t.Errorf("Province = %q, want Redmond", in.Province)
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
