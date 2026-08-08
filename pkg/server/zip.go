package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var zipCacheMutex sync.Mutex
var zipCache struct {
	dir   string
	mtime time.Time
	idx   *zipIndex
}

// serveZip 提供 seekable ZIP 下载：预建索引得到精确 Content-Length，
// 通过 http.ServeContent 支持 Range 断点续传与 HEAD。内存恒定。
func (s *Server) serveZip(w http.ResponseWriter, r *http.Request, dir string) {
	info, err := os.Stat(dir)
	if err != nil {
		s.writeNotFound(w, r, http.StatusInternalServerError, "500 无法打包目录："+err.Error())
		return
	}
	mtime := info.ModTime()

	zipCacheMutex.Lock()
	if zipCache.dir != dir || !zipCache.mtime.Equal(mtime) {
		idx, err := buildZipIndex(dir)
		if err != nil {
			zipCacheMutex.Unlock()
			s.writeNotFound(w, r, http.StatusInternalServerError, "500 无法打包目录："+err.Error())
			return
		}
		zipCache.dir = dir
		zipCache.mtime = mtime
		zipCache.idx = idx
	}
	idx := zipCache.idx
	zipCacheMutex.Unlock()

	zs := idx.newStream()
	defer zs.Close()
	name := filepath.Base(dir) + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("X-Lan-Share", "true")
	http.ServeContent(w, r, name, idx.modTime, zs)
}
