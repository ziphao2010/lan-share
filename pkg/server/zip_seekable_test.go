package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// 构造测试目录：a.txt(5B) + sub/b.bin(4B) + 空目录 empty/
func mkZipFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644)
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "b.bin"), []byte("beta"), 0o644)
	os.Mkdir(filepath.Join(root, "empty"), 0o755)
	return root
}

func readAllIndex(t *testing.T, idx *zipIndex) []byte {
	t.Helper()
	zs := idx.newStream()
	defer zs.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zs); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	return buf.Bytes()
}

// 核心：seekable zip 全量字节必须能被标准 zip 解析器完整解包（含 CRC 校验）。
func TestSeekableZipValidArchive(t *testing.T) {
	root := mkZipFixture(t)
	idx, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	data := readAllIndex(t, idx)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if len(zr.File) != 4 { // a.txt, sub/, sub/b.bin, empty/
		t.Errorf("entries = %d, want 4", len(zr.File))
	}
	names := map[string]string{}
	for _, f := range zr.File {
		if f.Mode().IsDir() {
			continue
		}
		fr, err := f.Open() // archive/zip 在读结束时校验 CRC
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(fr)
		fr.Close()
		if err != nil {
			t.Fatalf("read %s (crc error?): %v", f.Name, err)
		}
		names[f.Name] = string(b)
	}
	if names["a.txt"] != "alpha" || names["sub/b.bin"] != "beta" {
		t.Errorf("content mismatch: %v", names)
	}
}

// Content-Length 必须等于索引声明的总大小。
func TestSeekableZipContentLength(t *testing.T) {
	root := mkZipFixture(t)
	srv := New(root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?zip=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	cl := rec.Header().Get("Content-Length")
	if cl == "" {
		t.Fatal("missing Content-Length for zip")
	}
	idx, _ := buildZipIndex(root)
	zs := idx.newStream()
	if want := zs.Size(); cl != strconv.FormatInt(want, 10) {
		t.Errorf("Content-Length = %s, want %d", cl, want)
	}
}

// Range 分段下载后拼接必须等于全量字节（断点续传语义）。
func TestSeekableZipRangeResume(t *testing.T) {
	root := mkZipFixture(t)
	idx, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	full := readAllIndex(t, idx)

	// 在数据中部切一刀
	start := int64(len(full) / 3)
	srv := New(root)
	req := httptest.NewRequest(http.MethodGet, "/?zip=1", nil)
	req.Header.Set("Range", "bytes="+strconv.FormatInt(start, 10)+"-")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("code = %d, want 206", rec.Code)
	}
	tail := rec.Body.Bytes()
	joined := append(append([]byte{}, full[:start]...), tail...)
	if !bytes.Equal(joined, full) {
		t.Error("resumed bytes do not match full archive")
	}
}

// 大文件（>4GB zip64 分支）无法在测试里真实生成，改用“超量条目布局”模拟：
// 仅验证索引对任意 size 的偏移计算不溢出。
func TestZipIndexOffsetMath(t *testing.T) {
	entries := []zipEntry{
		{name: "huge.bin", size: 5 << 30}, // 5 GiB > 4GiB，触发 zip64 布局
	}
	idx := &zipIndex{entries: entries}
	idx.layout()
	if idx.entries[0].dataOffset == 0 {
		t.Fatal("layout not applied")
	}
	if idx.entries[0].dataOffset < int64(len(entries[0].hdr)) {
		t.Error("dataOffset should follow local header")
	}
	if idx.size <= 5<<30 {
		t.Errorf("size = %d, want > 5GiB", idx.size)
	}
}
