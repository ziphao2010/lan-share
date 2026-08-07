// Package server 提供内网文件分享的 HTTP 服务核心。
package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var errForbidden = errors.New("path escapes share root")

// NextPort 从 start 起尝试绑定，返回第一个可用端口。
// 探针绑定 127.0.0.1：Windows 上 :port（0.0.0.0）会因 SO_REUSEADDR
// 掩盖已有的 127.0.0.1 监听，导致误报端口空闲。
func NextPort(start, attempts int) (int, error) {
	for i := 0; i < attempts; i++ {
		port := start + i
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("ports %d-%d are all in use", start, start+attempts-1)
}

// safeJoin 把 URL 路径映射到 root 内的物理路径，阻止路径穿越。
func safeJoin(root, urlPath string) (string, error) {
	dec, err := url.PathUnescape(urlPath)
	if err != nil {
		return "", errForbidden
	}
	segs := strings.Split(strings.ReplaceAll(dec, `\`, "/"), "/")
	full := root
	for _, seg := range segs {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			return "", errForbidden
		}
		full = filepath.Join(full, seg)
	}
	return full, nil
}

// Server 是 http.Handler 最小骨架，后续任务填充。
type Server struct {
	root    string
	logger  *log.Logger
	secret  string
	zipMode bool
}

// SetSecret 启用 token 鉴权，secret 为空时关闭鉴权。
func (s *Server) SetSecret(secret string) {
	s.secret = secret
}

// SetZipMode 使目录根路径直接以 zip 包响应，无需 ?zip=1 参数。
func (s *Server) SetZipMode(on bool) {
	s.zipMode = on
}

func New(root string) *Server {
	return &Server{
		root:   root,
		logger: log.New(os.Stderr, "[lan-share] ", log.LstdFlags),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.secret != "" && !s.checkToken(r) {
		s.writeNotFound(w, r, http.StatusForbidden, "403 禁止访问：令牌无效或已过期")
		return
	}
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range, If-None-Match, If-Modified-Since")
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodGet, http.MethodHead:
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		s.writeNotFound(w, r, http.StatusMethodNotAllowed, "405 方法不允许")
		return
	}
	full, err := safeJoin(s.root, r.URL.Path)
	if err != nil {
		s.writeNotFound(w, r, http.StatusBadRequest, "400 非法路径")
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		s.writeNotFound(w, r, http.StatusNotFound, "404 文件不存在")
		return
	}
	if info.IsDir() {
		if s.zipMode || r.URL.Query().Get("zip") == "1" {
			s.serveZip(w, r, full)
			return
		}
		s.serveDirList(w, r, full)
		return
	}
	s.serveFile(w, r, full)
}

// writeNotFound 输出中文错误页。
func (s *Server) writeNotFound(w http.ResponseWriter, r *http.Request, code int, msg string) {
	title := ""
	switch code {
	case http.StatusNotFound:
		title = "文件不存在"
	case http.StatusForbidden:
		title = "禁止访问"
	case http.StatusBadRequest:
		title = "非法请求"
	case http.StatusMethodNotAllowed:
		title = "方法不允许"
	default:
		title = "错误"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, `<!DOCTYPE html><html lang="zh"><head><meta charset="utf-8"><title>%d %s</title></head><body><h1>%d %s</h1><p>%s</p><p><a href="/">返回首页</a></p></body></html>`, code, title, code, title, msg)
}
