package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"lan-share/pkg/token"
)

func TestTokenProtectionRejectsMissing(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644)
	srv := New(root)
	srv.SetSecret("mysecret")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/f.txt", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403 for missing token", rec.Code)
	}
}

func TestTokenProtectionAllowsValid(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644)
	srv := New(root)
	srv.SetSecret("mysecret")

	exp := time.Now().Add(time.Hour)
	path := "/f.txt"
	u := "/f.txt?token=" + token.Generate("mysecret", path, exp) + "&exp=" + strconv.FormatInt(exp.Unix(), 10)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestNoSecretNoAuth(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644)
	srv := New(root) // no secret
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/f.txt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 without secret", rec.Code)
	}
}