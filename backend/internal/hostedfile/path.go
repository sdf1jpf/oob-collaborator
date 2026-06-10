package hostedfile

import (
	"errors"
	"path"
	"strings"
)

var ErrInvalidPath = errors.New("invalid hosted file path")

// NormalizePath validates and normalizes a hosted file URL path (e.g. "/evil.dtd").
func NormalizePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidPath
	}
	if strings.Contains(raw, "\x00") || strings.Contains(raw, "..") {
		return "", ErrInvalidPath
	}

	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}

	cleaned := path.Clean(raw)
	if cleaned == "/" || cleaned == "." {
		return "", ErrInvalidPath
	}
	if strings.Contains(cleaned, "..") {
		return "", ErrInvalidPath
	}

	name := strings.TrimPrefix(cleaned, "/")
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", ErrInvalidPath
		}
		if !validPathSegment(seg) {
			return "", ErrInvalidPath
		}
	}

	return cleaned, nil
}

func validPathSegment(seg string) bool {
	for _, c := range seg {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return len(seg) > 0
}

// ContentTypeFromPath guesses a MIME type from the file path extension.
func ContentTypeFromPath(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".dtd"):
		return "application/xml-dtd"
	case strings.HasSuffix(lower, ".xml"):
		return "application/xml"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "text/html"
	case strings.HasSuffix(lower, ".txt"):
		return "text/plain"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".css"):
		return "text/css"
	case strings.HasSuffix(lower, ".js"):
		return "application/javascript"
	default:
		return "application/octet-stream"
	}
}
