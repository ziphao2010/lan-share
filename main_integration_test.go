package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"lan-share/pkg/server"
)

// 真实 Handler 挂到 httptest.Server，验证直链全链路可用。
func TestServerRoundtrip(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("hello lan-share")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := server.New(dir)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(payload) {
		t.Errorf("body = %q, want %q", body, payload)
	}
}
