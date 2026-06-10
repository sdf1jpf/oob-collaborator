package token

import (
	"strings"
)

// ExtractSubDomain returns the leftmost label of a host/QNAME as the payload token.
// e.g. "abc123.yourdomain.com" -> "abc123"
func ExtractSubDomain(host, domain string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	domain = strings.ToLower(strings.TrimSpace(domain))

	if host == "" || domain == "" {
		return ""
	}

	if !strings.HasSuffix(host, domain) {
		return ""
	}

	prefix := strings.TrimSuffix(host, domain)
	prefix = strings.TrimSuffix(prefix, ".")
	if prefix == "" {
		return ""
	}

	labels := strings.Split(prefix, ".")
	return labels[0]
}

// FullHost builds the full collaborator hostname for a token.
func FullHost(token, domain string) string {
	return token + "." + domain
}
