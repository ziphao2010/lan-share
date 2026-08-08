package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeDirListsChildren(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("A"), 0o644)
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "b.md"), []byte("B"), 0o644)

	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "a.txt") || !strings.Contains(body, "sub/") {
		t.Error("dir listing missing children")
	}
	if !strings.Contains(body, "zip=1") {
		t.Error("listing missing zip link")
	}
}

func TestServeDirSortByNameDesc(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "z.txt"), []byte("Z"), 0o644)
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("A"), 0o644)
	os.WriteFile(filepath.Join(root, "m.txt"), []byte("M"), 0o644)

	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?sort=name&order=desc", nil))

	body := rec.Body.String()
	ia := strings.Index(body, "a.txt")
	im := strings.Index(body, "m.txt")
	iz := strings.Index(body, "z.txt")
	if ia == -1 || im == -1 || iz == -1 {
		t.Fatal("missing entries")
	}
	if !(iz < im && im < ia) {
		t.Errorf("expected desc order z>m>a, got positions z=%d m=%d a=%d", iz, im, ia)
	}
}

func TestServeDirSortBySizeAsc(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "big.txt"), []byte("0123456789"), 0o644) // 10 B
	os.WriteFile(filepath.Join(root, "small.txt"), []byte("ab"), 0o644)       // 2 B

	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?sort=size", nil))

	body := rec.Body.String()
	ib := strings.Index(body, "big.txt")
	is := strings.Index(body, "small.txt")
	if ib == -1 || is == -1 {
		t.Fatal("missing entries")
	}
	if !(is < ib) {
		t.Errorf("expected small before big, got small=%d big=%d", is, ib)
	}
}

func TestServeDirFilterByQuery(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("A"), 0o644)
	os.WriteFile(filepath.Join(root, "beta.log"), []byte("B"), 0o644)

	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?q=alp", nil))

	body := rec.Body.String()
	if !strings.Contains(body, "alpha.txt") {
		t.Error("filtered listing missing alpha.txt")
	}
	if strings.Contains(body, "beta.log") {
		t.Error("filtered listing should exclude beta.log")
	}
}
