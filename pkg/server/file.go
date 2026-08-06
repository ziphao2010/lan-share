package server

import (
	"net/http"
	"os"
	"path/filepath"
)

// serveFile 用 http.ServeContent 输出单个文件，自动处理 Range/If-Modified-Since。
// 底层 io.Copy 走 sendfile，内存固定 32KB buffer，不整文件载入。
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, full string) {
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "cannot stat", http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Lan-Share", "true")
	// ServeContent 要求请求 URL 的 Path 用于 Content-Type 推断文件的 name 无关，
	// 实际传文件基名即可（推断 Content-Type 用）。
	http.ServeContent(w, r, filepath.Base(full), info.ModTime(), f)
}