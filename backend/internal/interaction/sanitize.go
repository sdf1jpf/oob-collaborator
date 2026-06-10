package interaction

import "strings"

var sensitiveHeaderPrefixes = []string{
	"authorization",
	"proxy-authorization",
	"cookie",
	"set-cookie",
	"x-api-key",
	"x-auth-token",
}

// SanitizeHeaders redacts sensitive header values before persistence.
func SanitizeHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}
	out := make(map[string][]string, len(headers))
	for k, vals := range headers {
		if isSensitiveHeader(k) {
			redacted := make([]string, len(vals))
			for i := range vals {
				redacted[i] = "[REDACTED]"
			}
			out[k] = redacted
		} else {
			out[k] = vals
		}
	}
	return out
}

// SanitizeCookies redacts cookie values before persistence.
func SanitizeCookies(cookies map[string]string) map[string]string {
	if cookies == nil {
		return nil
	}
	out := make(map[string]string, len(cookies))
	for k := range cookies {
		out[k] = "[REDACTED]"
	}
	return out
}

func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range sensitiveHeaderPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+"-") {
			return true
		}
	}
	return false
}
