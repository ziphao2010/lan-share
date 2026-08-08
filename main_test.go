package main

import (
	"testing"
	"time"
)

func TestParseArgsDefaults(t *testing.T) {
	cfg, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Port)
	}
	if cfg.Secret != "" || cfg.ZipMode || cfg.Quiet {
		t.Error("disabled flags should default to false/empty")
	}
	if cfg.Path == "" {
		t.Error("path should default to cwd")
	}
}

func TestParseArgsWithFlags(t *testing.T) {
	cfg, err := parseArgs([]string{"-p", "9999", "-t", "2h", "-k", "s3cr3t", "--zip", "./dataset"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("port = %d, want 9999", cfg.Port)
	}
	if cfg.Expire != 2*time.Hour {
		t.Errorf("expire = %v, want 2h", cfg.Expire)
	}
	if cfg.Secret != "s3cr3t" {
		t.Errorf("secret = %q, want s3cr3t", cfg.Secret)
	}
	if !cfg.ZipMode {
		t.Error("zipMode must be true")
	}
	if cfg.Path != "./dataset" {
		t.Errorf("path = %q, want ./dataset", cfg.Path)
	}
}

func TestParseArgsRejectsMultiArgs(t *testing.T) {
	if _, err := parseArgs([]string{"a", "b"}); err == nil {
		t.Fatal("expected error for multiple path args")
	}
}
