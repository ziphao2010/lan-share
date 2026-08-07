package server

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// seekable ZIP 设计：
// 统一 ZIP64 + store(不压缩)。文件条目带 data descriptor(bit3)，
// 目录条目只有局部头（无数据、无 descriptor）。
// 构建索引时流式读取每个文件一次，算出 CRC 与精确字节偏移，
// 从而支持 Content-Length / Range 断点续传，且归档不驻留内存。

const (
	zip64Version       = 45
	localHeaderSig     = 0x04034b50
	dataDescriptorSig  = 0x08074b50
	centralHeaderSig   = 0x02014b50
	zip64EOCDSig       = 0x06064b50
	zip64EOCDLocSig    = 0x07064b50
	eocdSig            = 0x06054b50
	flagDataDescriptor = 0x0008
	extraZip64         = 0x0001
	localHdrLen        = 30
	centralHdrLen      = 46
	descriptorLen      = 24
	zip64EOCDLen       = 56
	zip64EOCDLocLen    = 20
	eocdLen            = 22
)

func extraLen() int   { return 4 + 16 } // tag2 + len2 + data16
func centralLen() int { return 4 + 24 } // tag2 + len2 + data24

type zipEntry struct {
	name       string // zip 内相对路径（/ 分隔）
	path       string // 物理路径
	isDir      bool
	size       int64
	crc        uint32
	dataOffset int64 // 数据区在流中的起始偏移
	hdr        []byte
}

type zipIndex struct {
	entries  []zipEntry
	size     int64
	cdOffset int64
	modTime  time.Time
}

// layout 计算每个条目的 dataOffset 与总大小。
func (idx *zipIndex) layout() {
	off := int64(0)
	for i := range idx.entries {
		e := &idx.entries[i]
		e.hdr = makeLocalHeader(e.name, e.size, e.isDir)
		e.dataOffset = off + int64(len(e.hdr))
		entryLen := int64(len(e.hdr)) + e.size
		if !e.isDir {
			entryLen += descriptorLen
		}
		off += entryLen
	}
	idx.cdOffset = off
	idx.size = off + idx.centralDirSize() + zip64EOCDLen + zip64EOCDLocLen + eocdLen
}

func (idx *zipIndex) centralDirSize() int64 {
	var n int64
	for i := range idx.entries {
		n += int64(centralHdrLen + len(idx.entries[i].name) + centralLen())
	}
	return n
}

func (idx *zipIndex) newStream() *zipStream {
	return &zipStream{idx: idx}
}

// makeLocalHeader 构造 ZIP64 store 本地头。
// 目录条目不写 flags(无 descriptor)，size 直接写 0，且不写 zip64 extra。
func makeLocalHeader(name string, size int64, isDir bool) []byte {
	mod := time.Now()
	flags := uint16(0)
	extra := 0
	if !isDir {
		flags = flagDataDescriptor
		extra = extraLen()
	}
	h := make([]byte, localHdrLen+extra+len(name))
	binary.LittleEndian.PutUint32(h[0:], localHeaderSig)
	binary.LittleEndian.PutUint16(h[4:], zip64Version)
	binary.LittleEndian.PutUint16(h[6:], flags)
	binary.LittleEndian.PutUint16(h[8:], 0) // store
	putDosTime(h[10:], mod)
	if isDir {
		binary.LittleEndian.PutUint32(h[14:], 0)
		binary.LittleEndian.PutUint32(h[18:], 0)
		binary.LittleEndian.PutUint32(h[22:], 0)
	} else {
		binary.LittleEndian.PutUint32(h[14:], 0)          // crc 在 descriptor
		binary.LittleEndian.PutUint32(h[18:], 0xffffffff) // csize
		binary.LittleEndian.PutUint32(h[22:], 0xffffffff) // usize
	}
	binary.LittleEndian.PutUint16(h[26:], uint16(len(name)))
	binary.LittleEndian.PutUint16(h[28:], uint16(extra))
	copy(h[30:], name)
	if !isDir {
		ext := h[30+len(name):]
		binary.LittleEndian.PutUint16(ext[0:], extraZip64)
		binary.LittleEndian.PutUint16(ext[2:], 16)
		binary.LittleEndian.PutUint64(ext[4:], uint64(size))  // usize
		binary.LittleEndian.PutUint64(ext[12:], uint64(size)) // csize
	}
	return h
}

func putDosTime(b []byte, t time.Time) {
	t = t.Local()
	var t16, d16 uint16
	t16 = uint16(t.Hour())<<11 | uint16(t.Minute())<<5 | uint16(t.Second()/2)
	d16 = uint16(t.Year()-1980)<<9 | uint16(t.Month())<<5 | uint16(t.Day())
	binary.LittleEndian.PutUint16(b[0:], t16)
	binary.LittleEndian.PutUint16(b[2:], d16)
}

// buildZipIndex 遍历目录构建索引；文件内容流式读取一次计算 CRC。
func buildZipIndex(dir string) (*zipIndex, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	idx := &zipIndex{modTime: info.ModTime()}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
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
		fi, err := d.Info()
		if err != nil {
			return err
		}
		e := zipEntry{
			name:  rel,
			path:  p,
			isDir: d.IsDir(),
			size:  fi.Size(),
		}
		if e.isDir && !strings.HasSuffix(e.name, "/") {
			e.name += "/"
		}
		if !d.IsDir() {
			e.crc, err = fileCRC(p)
			if err != nil {
				return err
			}
		}
		idx.entries = append(idx.entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(idx.entries, func(i, j int) bool {
		if idx.entries[i].isDir != idx.entries[j].isDir {
			return idx.entries[i].isDir
		}
		return idx.entries[i].name < idx.entries[j].name
	})
	idx.layout()
	return idx, nil
}

func fileCRC(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	h := crc32.NewIEEE()
	if _, err := io.Copy(h, f); err != nil {
		return 0, err
	}
	return h.Sum32(), nil
}

// zipStream 把 zipIndex 暴露为 io.ReadSeeker。
type zipStream struct {
	idx     *zipIndex
	pos     int64
	cur     int
	f       *os.File
	fpos    int64
	base    string // zs.f 对应的 entry.path
	central []byte
}

func (zs *zipStream) Size() int64 { return zs.idx.size }

func (zs *zipStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if zs.pos >= zs.idx.size {
		return 0, io.EOF
	}
	total := 0
	for total < len(p) && zs.pos < zs.idx.size {
		// 推进到 pos 所在的条目；纯目录条目无数据，立即越过
		for zs.cur < len(zs.idx.entries) {
			e := &zs.idx.entries[zs.cur]
			end := e.dataOffset + e.size + entryTail(e)
			if zs.pos < end {
				break
			}
			zs.cur++
		}
		if zs.cur >= len(zs.idx.entries) {
			// 中央目录区
			if zs.central == nil {
				zs.central = zs.centralBytes()
			}
			off := zs.pos - zs.idx.cdOffset
			if uint64(off) >= uint64(len(zs.central)) {
				break
			}
			n := copy(p[total:], zs.central[off:])
			total += n
			zs.pos += int64(n)
			break
		}
		e := &zs.idx.entries[zs.cur]
		headerStart := e.dataOffset - int64(len(e.hdr))
		switch {
		case zs.pos < e.dataOffset: // header 区
			n := copy(p[total:], e.hdr[zs.pos-headerStart:])
			total += n
			zs.pos += int64(n)
		case zs.pos < e.dataOffset+e.size: // 数据区
			n, err := zs.readFromFile(p[total:])
			total += n
			zs.pos += int64(n)
			if err != nil && err != io.EOF {
				return total, err
			}
		default: // descriptor 区（仅文件有）
			desc := makeDescriptor(e)
			off := zs.pos - (e.dataOffset + e.size)
			n := copy(p[total:], desc[off:])
			total += n
			zs.pos += int64(n)
		}
		// 越过当前条目
		if zs.pos >= e.dataOffset+e.size+entryTail(e) {
			zs.cur++
		}
	}
	if total == 0 {
		return 0, io.EOF
	}
	return total, nil
}

func entryTail(e *zipEntry) int64 {
	if e.isDir {
		return 0
	}
	return descriptorLen
}

func makeDescriptor(e *zipEntry) []byte {
	b := make([]byte, descriptorLen)
	binary.LittleEndian.PutUint32(b[0:], dataDescriptorSig)
	binary.LittleEndian.PutUint32(b[4:], e.crc)
	binary.LittleEndian.PutUint64(b[8:], uint64(e.size))
	binary.LittleEndian.PutUint64(b[16:], uint64(e.size))
	return b
}

func (zs *zipStream) readFromFile(p []byte) (int, error) {
	e := &zs.idx.entries[zs.cur]
	fileOff := zs.pos - e.dataOffset
	if zs.f == nil || zs.fpos != fileOff || zs.base != e.path {
		zs.closeFile()
		f, err := os.Open(e.path)
		if err != nil {
			return 0, err
		}
		if _, err := f.Seek(fileOff, io.SeekStart); err != nil {
			f.Close()
			return 0, err
		}
		zs.f = f
		zs.fpos = fileOff
		zs.base = e.path
	}
	maxRead := e.dataOffset + e.size - zs.pos
	if int64(len(p)) > maxRead {
		p = p[:maxRead]
	}
	n, err := zs.f.Read(p)
	zs.fpos += int64(n)
	return n, err
}

func (zs *zipStream) closeFile() {
	if zs.f != nil {
		zs.f.Close()
		zs.f = nil
	}
	zs.base = ""
}

func (zs *zipStream) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = zs.pos + offset
	case io.SeekEnd:
		newPos = zs.idx.size + offset
	default:
		return 0, os.ErrInvalid
	}
	if newPos < 0 {
		return 0, os.ErrInvalid
	}
	zs.closeFile()
	zs.pos = newPos
	zs.cur = 0
	return newPos, nil
}

func (zs *zipStream) Close() error {
	zs.closeFile()
	return nil
}

func (zs *zipStream) getModTime() time.Time { return zs.idx.modTime }

// centralBytes 构造中央目录 + zip64 EOCD + locator + EOCD。
func (zs *zipStream) centralBytes() []byte {
	var b []byte
	for i := range zs.idx.entries {
		b = append(b, zs.centralEntry(&zs.idx.entries[i])...)
	}
	b = append(b, zs.zip64EOCD()...)
	b = append(b, zs.zip64EOCDLocator()...)
	b = append(b, zs.eocd()...)
	return b
}

func (zs *zipStream) centralEntry(e *zipEntry) []byte {
	ln := len(e.name)
	b := make([]byte, centralHdrLen+centralLen()+ln)
	binary.LittleEndian.PutUint32(b[0:], centralHeaderSig)
	binary.LittleEndian.PutUint16(b[4:], 0x031e) // made by: unix | 45
	binary.LittleEndian.PutUint16(b[6:], zip64Version)
	binary.LittleEndian.PutUint16(b[8:], 0) // 不含 data descriptor（合法）
	binary.LittleEndian.PutUint16(b[10:], 0) // store
	putDosTime(b[12:], zs.getModTime())
	binary.LittleEndian.PutUint32(b[16:], e.crc)
	if e.isDir {
		binary.LittleEndian.PutUint32(b[20:], 0)
		binary.LittleEndian.PutUint32(b[24:], 0)
	} else {
		binary.LittleEndian.PutUint32(b[20:], 0xffffffff)
		binary.LittleEndian.PutUint32(b[24:], 0xffffffff)
	}
	binary.LittleEndian.PutUint16(b[28:], uint16(ln))
	binary.LittleEndian.PutUint16(b[30:], uint16(centralLen()))
	binary.LittleEndian.PutUint16(b[32:], 0) // comment len
	binary.LittleEndian.PutUint16(b[34:], 0) // disk start
	binary.LittleEndian.PutUint16(b[36:], 0) // internal attrs
	if e.isDir {
		binary.LittleEndian.PutUint32(b[38:], 0x0010)
	} else {
		binary.LittleEndian.PutUint32(b[38:], 0)
	}
	binary.LittleEndian.PutUint32(b[42:], 0xffffffff) // local header offset (zip64)
	copy(b[centralHdrLen:], e.name)
	extra := b[centralHdrLen+ln:]
	binary.LittleEndian.PutUint16(extra[0:], extraZip64)
	binary.LittleEndian.PutUint16(extra[2:], 24)
	binary.LittleEndian.PutUint64(extra[4:], uint64(e.size))
	binary.LittleEndian.PutUint64(extra[12:], uint64(e.size))
	binary.LittleEndian.PutUint64(extra[20:], uint64(e.dataOffset-int64(len(e.hdr))))
	return b
}

// zip64EOCD 记录（56B）。
func (zs *zipStream) zip64EOCD() []byte {
	b := make([]byte, zip64EOCDLen)
	binary.LittleEndian.PutUint32(b[0:], zip64EOCDSig)
	binary.LittleEndian.PutUint64(b[4:], 44)
	binary.LittleEndian.PutUint16(b[12:], 0x031e)
	binary.LittleEndian.PutUint16(b[14:], zip64Version)
	binary.LittleEndian.PutUint32(b[16:], 0) // 本盘号
	binary.LittleEndian.PutUint32(b[20:], 0) // CD 起始盘号
	binary.LittleEndian.PutUint64(b[24:], uint64(len(zs.idx.entries)))
	binary.LittleEndian.PutUint64(b[32:], uint64(len(zs.idx.entries)))
	binary.LittleEndian.PutUint64(b[40:], uint64(zs.idx.centralDirSize())) // CD 大小
	binary.LittleEndian.PutUint64(b[48:], uint64(zs.idx.cdOffset))         // CD 偏移
	return b
}

func (zs *zipStream) zip64EOCDLocator() []byte {
	b := make([]byte, zip64EOCDLocLen)
	binary.LittleEndian.PutUint32(b[0:], zip64EOCDLocSig)
	binary.LittleEndian.PutUint32(b[4:], 0) // 含 zip64 EOCD 的盘号
	binary.LittleEndian.PutUint64(b[8:], uint64(zs.idx.cdOffset+zs.idx.centralDirSize()))
	binary.LittleEndian.PutUint32(b[16:], 1) // 总盘数
	return b
}

func (zs *zipStream) eocd() []byte {
	b := make([]byte, eocdLen)
	binary.LittleEndian.PutUint32(b[0:], eocdSig)
	binary.LittleEndian.PutUint16(b[4:], 0xffff)
	binary.LittleEndian.PutUint16(b[6:], 0xffff)
	binary.LittleEndian.PutUint16(b[8:], 0xffff)
	binary.LittleEndian.PutUint16(b[10:], 0xffff)
	binary.LittleEndian.PutUint32(b[12:], 0xffffffff)
	binary.LittleEndian.PutUint32(b[16:], 0xffffffff)
	binary.LittleEndian.PutUint16(b[20:], 0)
	return b
}