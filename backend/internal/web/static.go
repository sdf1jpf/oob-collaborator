package web

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

const embedRoot = "/embed"

func StaticHandler(distPath string) http.Handler {
	absDist, err := filepath.Abs(distPath)
	if err != nil {
		return notFoundHandler("frontend not built")
	}
	if _, err := os.Stat(absDist); err != nil {
		return notFoundHandler("frontend not built")
	}

	fileServer := http.FileServer(http.Dir(absDist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReservedPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		target, ok := safeFileUnder(absDist, r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		if f, err := os.Stat(target); err == nil && !f.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		indexPath, ok := safeFileUnder(absDist, "/index.html")
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
		http.NotFound(w, r)
	})
}

// EmbedFSHandler serves from an embedded filesystem.
func EmbedFSHandler(content fs.FS) http.Handler {
	sub, err := fs.Sub(content, ".")
	if err != nil {
		return notFoundHandler("frontend not available")
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReservedPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		path, ok := safeRelativePath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(sub, path); err != nil {
			urlCopy := *r.URL
			urlCopy.Path = "/index.html"
			r2 := *r
			r2.URL = &urlCopy
			fileServer.ServeHTTP(w, &r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func isReservedPath(path string) bool {
	return strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/bi/") ||
		strings.HasPrefix(path, "/ws")
}

// safeFileUnder resolves requestPath under base using SecureJoin.
func safeFileUnder(base, requestPath string) (string, bool) {
	rel := strings.TrimPrefix(requestPath, "/")
	full, err := securejoin.SecureJoin(base, rel)
	if err != nil {
		return "", false
	}
	return full, true
}

// safeRelativePath resolves a URL path to a safe relative path for fs.FS.
func safeRelativePath(requestPath string) (string, bool) {
	rel := strings.TrimPrefix(requestPath, "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	joined, err := securejoin.SecureJoin(embedRoot, rel)
	if err != nil {
		return "", false
	}
	path := strings.TrimPrefix(joined, embedRoot)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "index.html", true
	}
	if strings.Contains(path, "..") {
		return "", false
	}
	return path, true
}

func notFoundHandler(msg string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, msg, http.StatusNotFound)
	})
}
