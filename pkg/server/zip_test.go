package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeZipStreamsDirectory(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644)
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "b.bin"), []byte("beta"), 0o644)

	srv := New(root)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?zip=1", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "zip") {
		t.Errorf("Content-Type = %q, want zip", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	names := map[string]string{}
	for _, f := range zr.File {
		fr, _ := f.Open()
		b, _ := io.ReadAll(fr)
		fr.Close()
		names[f.Name] = string(b)
	}
	if names["a.txt"] != "alpha" {
		t.Errorf("a.txt = %q, want alpha", names["a.txt"])
	}
	if names["sub/b.bin"] != "beta" {
		t.Errorf("sub/b.bin = %q, want beta", names["sub/b.bin"])
	}
}

func TestServeZipModeWithoutQuery(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644)

	srv := New(root)
	srv.SetZipMode(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "zip") {
		t.Errorf("Content-Type = %q, want zip in zip mode", ct)
	}
}