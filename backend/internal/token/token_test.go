package token

import "testing"

func TestExtractSubDomain(t *testing.T) {
	domain := "example.com"
	cases := []struct {
		host string
		want string
	}{
		{"abc123.example.com", "abc123"},
		{"abc123.example.com.", "abc123"},
		{"deep.abc123.example.com", "deep"},
		{"example.com", ""},
		{"other.com", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := ExtractSubDomain(tc.host, domain)
		if got != tc.want {
			t.Errorf("ExtractSubDomain(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestFullHost(t *testing.T) {
	if got := FullHost("tok", "example.com"); got != "tok.example.com" {
		t.Fatalf("FullHost = %q", got)
	}
}
