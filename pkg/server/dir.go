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

var dirTmpl = template.Must(template.New("dir").Parse(dirPage))

type dirEntry struct {
	Name      string
	Size      string
	SizeBytes int64
	Mod       time.Time
	URL       string
}

func (s *Server) serveDirList(w http.ResponseWriter, r *http.Request, dir string) {
	q := r.URL.Query()
	sortKey := q.Get("sort")
	order := q.Get("order")
	query := q.Get("q")

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
		if query != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			continue
		}
		disp := name
		if e.IsDir() {
			disp += "/"
		}
		href := strings.TrimSuffix(r.URL.Path, "/") + "/" + url.PathEscape(name)
		items = append(items, dirEntry{
			Name:      disp,
			Size:      HumanSize(info.Size()),
			SizeBytes: info.Size(),
			Mod:       info.ModTime(),
			URL:       href,
		})
	}
	if r.URL.Path != "/" && r.URL.Path != "" {
		parentURL := strings.TrimRight(r.URL.Path, "/")
		if idx := strings.LastIndex(parentURL, "/"); idx >= 0 {
			parentURL = parentURL[:idx+1]
		} else {
			parentURL = "/"
		}
		items = append([]dirEntry{{Name: "..", URL: parentURL}}, items...)
	}
	sort.Slice(items, func(i, j int) bool {
		less := false
		switch sortKey {
		case "size":
			less = items[i].SizeBytes < items[j].SizeBytes
		case "time":
			less = items[i].Mod.Before(items[j].Mod)
		default:
			less = items[i].Name < items[j].Name
		}
		if order == "desc" {
			return !less
		}
		return less
	})

	data := struct {
		Path    string
		Items   []dirEntry
		ZipLink string
		Q       string
		Sort    string
		Order   string
	}{
		Path:    r.URL.Path,
		Items:   items,
		ZipLink: strings.TrimSuffix(r.URL.Path, "/") + "/?zip=1",
		Q:       query,
		Sort:    sortKey,
		Order:   order,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := dirTmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
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
<form method="get" action="{{.Path}}">
  <input type="text" name="q" value="{{.Q}}" placeholder="搜索文件…">
  <select name="sort">
    <option value="name" {{if eq .Sort "name"}}selected{{end}}>按名称</option>
    <option value="size" {{if eq .Sort "size"}}selected{{end}}>按大小</option>
    <option value="time" {{if eq .Sort "time"}}selected{{end}}>按时间</option>
  </select>
  <select name="order">
    <option value="asc" {{if ne .Order "desc"}}selected{{end}}>升序</option>
    <option value="desc" {{if eq .Order "desc"}}selected{{end}}>降序</option>
  </select>
  <button type="submit">筛选</button>
</form>
<table>
<tr><th>名称</th><th>大小</th><th>修改时间</th></tr>
{{range .Items}}<tr><td><a href="{{.URL}}">{{.Name}}</a></td><td>{{.Size}}</td><td>{{.Mod.Format "2006-01-02 15:04"}}</td></tr>{{end}}
</table>
</body></html>`
