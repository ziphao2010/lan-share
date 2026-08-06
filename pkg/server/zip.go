package server

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// serveZip 流式打包目录，边遍历边写入 zip，内存占用恒定。
func (s *Server) serveZip(w http.ResponseWriter, dir string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, filepath.Base(dir)))
	zw := zip.NewWriter(w)
	err := filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			_, err := zw.Create(rel + "/")
			return err
		}
		fh, err := zw.Create(rel)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(fh, src)
		return err
	})
	if err != nil {
		// zip 已写入部分字节到响应体，无法改状态码；记录日志（真实场景用 log）。
		s.logger.Printf("zip error: %v", err)
	}
	zw.Close()
}