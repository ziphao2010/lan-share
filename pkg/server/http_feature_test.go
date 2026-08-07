package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileETagAndIfNoneMatch(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello"), 0o644)

	srv := New(root)
	// 首次请求，应带 ETag
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/f.txt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Error("missing Cache-Control on file response")
	}

	// 带 If-None-Match 再次请求，应 304
	req := httptest.NewRequest(http.MethodGet, "/f.txt", nil)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("code = %d, want 304", rec2.Code)
	}
}

func TestNotFoundChinesePage(t *testing.T) {
	srv := New(t.TempDir())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope.bin", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "404") || !strings.Contains(body, "不存在") {
		t.Errorf("404 page should be Chinese, got: %s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "html") {
		t.Errorf("Content-Type = %q, want html", ct)
	}
}

func TestOptionsReturnsAllowed(t *testing.T) {
	srv := New(t.TempDir())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/f.txt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") || !strings.Contains(allow, "OPTIONS") {
		t.Errorf("Allow = %q, want GET/HEAD/OPTIONS", allow)
	}
}

func TestDirListingCacheControl(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("A"), 0o644)
	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Cache-Control") == "" {
		t.Error("dir listing should have Cache-Control: no-cache")
	}
}

func TestAccessLogWritesEntries(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello"), 0o644)

	var buf bytes.Buffer
	srv := New(root)
	srv.SetLogger(log.New(&buf, "", 0))
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/f.txt", nil))

	out := buf.String()
	if !strings.Contains(out, "GET /f.txt") {
		t.Errorf("log should contain method+path, got: %s", out)
	}
	if !strings.Contains(out, "200") {
		t.Errorf("log should contain status 200, got: %s", out)
	}
	if !strings.Contains(out, "5B") {
		t.Errorf("log should contain bytes=5B, got: %s", out)
	}
	if !strings.Contains(out, "ms") {
		t.Errorf("log should contain duration, got: %s", out)
	}
}