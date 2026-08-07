package server

import (
	"fmt"
	"net/http"
	"path/filepath"
)

// serveZip 提供 seekable ZIP 下载：预建索引得到精确 Content-Length，
// 通过 http.ServeContent 支持 Range 断点续传与 HEAD。内存恒定。
func (s *Server) serveZip(w http.ResponseWriter, r *http.Request, dir string) {
	idx, err := buildZipIndex(dir)
	if err != nil {
		s.writeNotFound(w, r, http.StatusInternalServerError, "500 无法打包目录："+err.Error())
		return
	}
	zs := idx.newStream()
	defer zs.Close()
	name := filepath.Base(dir) + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("X-Lan-Share", "true")
	http.ServeContent(w, r, name, idx.modTime, zs)
}