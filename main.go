package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"lan-share/pkg/ipaddr"
	"lan-share/pkg/server"
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

func run(cfg *Config) error {
	abs, err := filepath.Abs(cfg.Path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%s 不存在: %v", abs, err)
	}
	port, err := server.NextPort(cfg.Port, 10)
	if err != nil {
		return err
	}
	root := abs
	if !info.IsDir() {
		root = filepath.Dir(abs)
	}

	srv := server.New(root)
	if cfg.Secret != "" {
		srv.SetSecret(cfg.Secret)
	}
	srv.SetZipMode(cfg.ZipMode)

	var expire time.Time
	if cfg.Expire > 0 {
		expire = time.Now().Add(cfg.Expire)
	}

	rel := "/"
	if !info.IsDir() {
		rel = "/" + filepath.Base(abs)
	}

	ips := ipaddr.ListLocalIPs()
	if cfg.IP != "" {
		ips = []string{cfg.IP}
	}

	if !cfg.Quiet {
		if info.IsDir() {
			fmt.Printf("分享目录：%s\n", abs)
		} else {
			fmt.Printf("分享文件：%s (%s)\n", abs, server.HumanSize(info.Size()))
		}
		fmt.Println("直链:")
	}
	for _, ip := range ips {
		u := srv.LinkURL(ip, port, rel, expire)
		fmt.Println("  " + u)
	}
	if !cfg.Quiet {
		if expire.IsZero() {
			fmt.Println("过期时间：永久")
		} else {
			fmt.Println("过期时间：" + expire.Format("2006-01-02 15:04"))
		}
		fmt.Println("访问保护：" + flagOn(cfg.Secret))
		fmt.Println("Ctrl+C 停止服务")
	}

	handler := http.Server{
		Addr:    cfg.Bind + ":" + strconv.Itoa(port),
		Handler: srv,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
		return nil
	}
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "lan-share:", err)
		os.Exit(1)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "lan-share:", err)
		os.Exit(1)
	}
}

func flagOn(s string) string {
	if s == "" {
		return "无"
	}
	return "开启"
}
