package server

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type dirEntry struct {
	Name string
	Size string
	Mod  time.Time
	URL  string
}

func (s *Server) serveDirList(w http.ResponseWriter, r *http.Request, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "cannot read directory", http.StatusInternalServerError)
		return
	}
	var items []dirEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		disp := name
		if e.IsDir() {
			disp += "/"
		}
		href := strings.TrimSuffix(r.URL.Path, "/") + "/" + url.PathEscape(name)
		items = append(items, dirEntry{
			Name: disp,
			Size: HumanSize(info.Size()),
			Mod:  info.ModTime(),
			URL:  href,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	tmpl := template.Must(template.New("dir").Parse(dirPage))
	data := struct {
		Path    string
		Items   []dirEntry
		ZipLink string
	}{
		Path:    r.URL.Path,
		Items:   items,
		ZipLink: strings.TrimSuffix(r.URL.Path, "/") + "/?zip=1",
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

// HumanSize 输出人类可读大小（如 4.2 GiB）。
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

const dirPage = `<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>indices - {{.Path}}</title></head>
<body>
<h1>目录 {{.Path}}</h1>
<p><a href="{{.ZipLink}}">下载全部为 ZIP</a></p>
<table>
<tr><th>名称</th><th>大小</th><th>修改时间</th></tr>
{{range .Items}}<tr><td><a href="{{.URL}}">{{.Name}}</a></td><td>{{.Size}}</td><td>{{.Mod.Format "2006-01-02 15:04"}}</td></tr>{{end}}
</table>
</body></html>`