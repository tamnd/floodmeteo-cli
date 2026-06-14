package floodmeteo

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// Client HTTP behaviour is covered in floodmeteo_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "floodmeteo" {
		t.Errorf("Scheme = %q, want floodmeteo", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != "flood-api.open-meteo.com" {
		t.Errorf("Hosts = %v, want [flood-api.open-meteo.com]", info.Hosts)
	}
	if info.Identity.Binary != "floodmeteo" {
		t.Errorf("Identity.Binary = %q, want floodmeteo", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"52.52,13.41", "latlon", "52.52,13.41"},
		{"52.52, 13.41", "latlon", "52.52, 13.41"},
		{"-33.87,151.21", "latlon", "-33.87,151.21"},
		{"berlin", "query", "berlin"},
		{"River Thames", "query", "River Thames"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)", tc.in, typ, id, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("latlon", "52.52,13.41")
	want := "https://flood-api.open-meteo.com/v1/flood?latitude=52.52&longitude=13.41"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateQuery(t *testing.T) {
	_, err := Domain{}.Locate("query", "berlin")
	if err != nil {
		t.Errorf("Locate query: unexpected error: %v", err)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "x")
	if err == nil {
		t.Error("Locate unknown type: expected error, got nil")
	}
}
