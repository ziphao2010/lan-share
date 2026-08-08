package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	w.Header().Set("Cache-Control", "public, max-age=0")
	etag := fmt.Sprintf(`"%x-%x"`, info.ModTime().UnixNano(), info.Size())
	w.Header().Set("ETag", etag)
	if ok := etagMatch(r.Header.Get("If-None-Match"), etag); ok {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	// ServeContent 要求请求 URL 的 Path 用于 Content-Type 推断文件的 name 无关，
	// 实际传文件基名即可（推断 Content-Type 用）。
	http.ServeContent(w, r, filepath.Base(full), info.ModTime(), f)
}

// etagMatch 解析 If-None-Match（可为 * 或多个 etag），判断是否命中。
func etagMatch(ifNoneMatch, current string) bool {
	if ifNoneMatch == "" {
		return false
	}
	vals := strings.Split(ifNoneMatch, ",")
	for i := range vals {
		v := strings.TrimSpace(vals[i])
		if v == "*" || v == current {
			return true
		}
	}
	return false
}
