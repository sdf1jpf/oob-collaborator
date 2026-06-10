package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeFileUnderCannotEscapeRoot(t *testing.T) {
	base := t.TempDir()
	absBase, err := filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}

	parent := filepath.Dir(absBase)
	secret := filepath.Join(parent, "oob-secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })

	traversalPaths := []string{
		"/../oob-secret.txt",
		"../oob-secret.txt",
		"/foo/../../oob-secret.txt",
		"../../../../oob-secret.txt",
		"..",
	}
	for _, p := range traversalPaths {
		got, ok := safeFileUnder(base, p)
		if !ok {
			continue
		}
		if !pathUnderBase(absBase, got) {
			t.Fatalf("path %q resolved outside base: %q", p, got)
		}
		if got == secret {
			t.Fatalf("path %q escaped to secret file", p)
		}
	}
}

func TestSafeFileUnderAllowsValidFile(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "assets", "app.js")
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := safeFileUnder(base, "/assets/app.js")
	if !ok {
		t.Fatal("expected valid path")
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSafeRelativePathCannotEscapeRoot(t *testing.T) {
	for _, p := range []string{
		"/../etc/passwd",
		"/..",
		"/foo/../../etc/passwd",
		"..",
		"../../../../etc/passwd",
	} {
		got, ok := safeRelativePath(p)
		if !ok {
			continue
		}
		if strings.Contains(got, "..") || strings.HasPrefix(got, "/") {
			t.Fatalf("path %q produced unsafe relative path %q", p, got)
		}
	}
}

func TestSafeRelativePathAllowsValidFile(t *testing.T) {
	got, ok := safeRelativePath("/assets/app.js")
	if !ok || got != "assets/app.js" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestStaticHandlerRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := StaticHandler(base)
	req := httptest.NewRequest(http.MethodGet, "/../../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 404 or 400", rec.Code)
	}
}

func pathUnderBase(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	return err == nil && !strings.HasPrefix(rel, "..")
}
