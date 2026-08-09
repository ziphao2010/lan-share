package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

var zipCacheMutex sync.Mutex
var zipCache struct {
	dir string
	idx *zipIndex
}

func (idx *zipIndex) changed(dir string) bool {
	byPath := make(map[string]int, len(idx.entries))
	for i := range idx.entries {
		byPath[idx.entries[i].path] = i
	}
	seen := 0
	changed := false
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			changed = true
			return nil
		}
		if p == dir {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			changed = true
			return nil
		}
		i, ok := byPath[p]
		if !ok {
			changed = true
			return nil
		}
		seen++
		e := &idx.entries[i]
		if e.isDir != d.IsDir() {
			changed = true
			return nil
		}
		if !e.isDir && e.size != fi.Size() {
			changed = true
			return nil
		}
		if !e.modTime.Equal(fi.ModTime()) {
			changed = true
		}
		return nil
	})
	if seen != len(idx.entries) {
		changed = true
	}
	return changed
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
	if zipCache.dir != dir || zipCache.idx == nil || zipCache.idx.changed(dir) {
		idx, err := buildZipIndex(dir)
		if err != nil {
			zipCacheMutex.Unlock()
			s.writeNotFound(w, r, http.StatusInternalServerError, "500 无法打包目录："+err.Error())
			return
		}
		zipCache.dir = dir
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
	http.ServeContent(w, r, name, mtime, zs)
}
