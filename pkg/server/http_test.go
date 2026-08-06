package server

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextPortSkipsBusy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot reserve port: %v", err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	got, err := NextPort(busy, 10)
	if err != nil {
		t.Fatalf("NextPort error: %v", err)
	}
	if got == busy {
		t.Errorf("got busy port %d", got)
	}
	if got != busy+1 {
		t.Errorf("got %d, want %d", got, busy+1)
	}
}

func TestSafeJoinBlocksTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := safeJoin(root, "/../etc/passwd"); err == nil {
		t.Error("expected error for .. traversal")
	}
	if _, err := safeJoin(root, "/%2e%2e/x"); err == nil {
		t.Error("expected error for encoded traversal")
	}
	if _, err := safeJoin(root, "/.."); err == nil {
		t.Error("expected error for bare ..")
	}
}

func TestSafeJoinWithinRoot(t *testing.T) {
	root := t.TempDir()
	sub, err := os.MkdirTemp(root, "d")
	if err != nil {
		t.Fatal(err)
	}
	full, err := safeJoin(root, "/"+filepath.Base(sub)+"/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(full, root) {
		t.Errorf("path %q escaped root %q", full, root)
	}
}
