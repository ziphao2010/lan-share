package server

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// 原地修改文件（size/mtime 变化）必须触发索引失效重建。
func TestZipIndexInvalidationOnInPlaceEdit(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte("alpha"), 0o644)

	idx1, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("alphax"), 0o644)
	if !idx1.changed(root) {
		t.Fatal("in-place edit should invalidate index")
	}

	idx2, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if idx2.changed(root) {
		t.Fatal("unchanged tree should not invalidate")
	}
}

// 目录条目 size 必须为 0（Linux 上目录 st_size=4096，会导致布局错乱）。
func TestZipIndexDirSizeIsZero(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	idx, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.entries {
		if e.isDir && e.size != 0 {
			t.Fatalf("dir %s size = %d, want 0", e.name, e.size)
		}
	}
	data := readAllIndex(t, idx)
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
}

// 同一目录两次构建必须字节完全一致（时间戳确定性，断点续传安全）。
func TestZipIndexDeterministicBuild(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644)
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "b.bin"), []byte("beta"), 0o644)

	idx1, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	idx2, err := buildZipIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	data1 := readAllIndex(t, idx1)
	data2 := readAllIndex(t, idx2)
	if !bytes.Equal(data1, data2) {
		t.Error("two builds of same tree must be byte-identical")
	}
}
