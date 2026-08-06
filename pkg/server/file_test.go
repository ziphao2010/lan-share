package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServeFileFullDownload(t *testing.T) {
	root := t.TempDir()
	data := make([]byte, 1<<20) // 1MB
	rand.Read(data)
	name := "blob.bin"
	if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(root)
	req := httptest.NewRequest(http.MethodGet, "/"+name, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Error("missing Accept-Ranges: bytes (breaks resume)")
	}
	if rec.Header().Get("X-Lan-Share") != "true" {
		t.Error("missing X-Lan-Share header")
	}
	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Error("body mismatch")
	}
}

func TestServeFileRangeResume(t *testing.T) {
	root := t.TempDir()
	data := []byte("0123456789abcdef")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(root)
	req := httptest.NewRequest(http.MethodGet, "/f.txt", nil)
	req.Header.Set("Range", "bytes=5-9")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("code = %d, want 206", rec.Code)
	}
	if got := rec.Body.String(); got != "56789" {
		t.Errorf("range body = %q, want %q", got, "56789")
	}
}

func TestServeFileNotFound(t *testing.T) {
	srv := New(t.TempDir())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope.bin", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestServeFileSHA256(t *testing.T) {
	root := t.TempDir()
	data := make([]byte, 32<<20) // 32MB
	rand.Read(data)
	name := "big.bin"
	if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(data)

	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+name, nil))

	got, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(got); sum != want {
		t.Error("sha256 mismatch")
	}
}