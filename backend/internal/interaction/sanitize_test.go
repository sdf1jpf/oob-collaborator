package interaction

import "testing"

func TestSanitizeHeadersRedactsSensitive(t *testing.T) {
	headers := map[string][]string{
		"Authorization": {"Bearer token"},
		"User-Agent":    {"curl/8.0"},
	}
	out := SanitizeHeaders(headers)
	if out["Authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected authorization redacted, got %q", out["Authorization"][0])
	}
	if out["User-Agent"][0] != "curl/8.0" {
		t.Fatalf("expected user-agent preserved, got %q", out["User-Agent"][0])
	}
}

func TestSanitizeCookiesRedactsValues(t *testing.T) {
	out := SanitizeCookies(map[string]string{"session": "abc"})
	if out["session"] != "[REDACTED]" {
		t.Fatalf("expected cookie redacted, got %q", out["session"])
	}
}
