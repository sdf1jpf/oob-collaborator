package security

import (
	"crypto/subtle"
	"fmt"
	"strings"
)

const (
	SessionCookieName = "oob_session"
	JWTIssuer         = "oob-collaborator"
	JWTAudience       = "oob-collaborator-admin"
)

var blockedSecrets = map[string]struct{}{
	"changeme":                          {},
	"dev-secret-change-me":              {},
	"dev-secret-change-me-in-production": {},
	"changeme-poll-token":               {},
	"change-this-to-a-long-random-string": {},
}

// ConstantTimeEqual compares two strings in constant time.
func ConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ValidateSecret rejects empty, blocked, or too-short secrets.
func ValidateSecret(name, value string, minLen int, devMode bool) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if devMode {
		return nil
	}
	if len(value) < minLen {
		return fmt.Errorf("%s must be at least %d characters in production", name, minLen)
	}
	if _, blocked := blockedSecrets[strings.ToLower(value)]; blocked {
		return fmt.Errorf("%s uses a known insecure default; set a strong random value", name)
	}
	return nil
}
