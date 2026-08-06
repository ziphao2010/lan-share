package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port    int
	Expire  time.Duration
	Secret  string
	ZipMode bool
	IP      string
	Bind    string
	Quiet   bool
	Path    string
}

func parseArgs(args []string) (*Config, error) {
	fs := flag.NewFlagSet("lan-share", flag.ContinueOnError)
	var cfg Config
	fs.IntVar(&cfg.Port, "p", 8080, "listen port (auto-increments if busy)")
	fs.IntVar(&cfg.Port, "port", 8080, "listen port (alias)")
	fs.DurationVar(&cfg.Expire, "t", 0, "expire duration (e.g. 2h, 30m, 7d); 0 = never")
	fs.StringVar(&cfg.Secret, "k", "", "access secret; generates token-protected URLs")
	fs.StringVar(&cfg.Secret, "token", "", "access secret (alias)")
	fs.BoolVar(&cfg.ZipMode, "zip", false, "serve directory as zip archive")
	fs.StringVar(&cfg.IP, "ip", "", "override advertised IP (auto-detect if empty)")
	fs.StringVar(&cfg.Bind, "bind", "0.0.0.0", "listen address")
	fs.BoolVar(&cfg.Quiet, "q", false, "quiet: only print URLs")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "quiet (alias)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	switch fs.NArg() {
	case 0:
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cfg.Path = cwd
	case 1:
		cfg.Path = fs.Arg(0)
	default:
		return nil, fmt.Errorf("expected at most one path argument, got %d", fs.NArg())
	}
	return &cfg, nil
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "lan-share:", err)
		os.Exit(1)
	}
	_ = cfg // TODO: wired up in later tasks
}