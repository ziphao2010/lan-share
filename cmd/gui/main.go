//go:build windows

// Lan Share GUI 启动器。
// 原生 Win32 窗口（纯标准库，无 cgo）：选择文件/文件夹、设置端口与可选
// token，一键启动 HTTP 服务并展示局域网直链，支持一键复制。
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"lan-share/pkg/ipaddr"
	"lan-share/pkg/server"
)

// 控件 ID
const (
	IDBtnFile  = 101
	IDBtnDir   = 102
	IDBtnStart = 103
	IDBtnCopy  = 104

	IDEditPath  = 201
	IDEditPort  = 202
	IDEditToken = 203
	IDEditLog   = 204
)

// Win32 常量
const (
	wndClassName = "LanShareGUIWnd"

	wsChild       = 0x40000000
	wsVisible     = 0x10000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsThickFrame  = 0x00040000
	wsMaximizeBox = 0x00010000
	wsVScroll     = 0x00200000
	wsTabsTop     = 0x00010000
	wsBorder      = 0x00800000

	bsPushButton  = 0x00000000
	esAutoHScroll = 0x00000080
	esMultiline   = 0x00000004
	esReadOnly    = 0x00000800
	esWantReturn  = 0x00001000

	cwUseDefault   = 0x80000000
	swShow         = 5
	IDCArrow       = 32512
	IDIApplication = 32512
	defaultGuiFont = 17
	maxPath        = 260

	wmCommand  = 0x0111
	wmClose    = 0x0010
	wmDestroy  = 0x0002
	wmSetfont  = 0x0030
	wmVscroll  = 0x0115
	sbBottom   = 7

	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040
	bifEditBox          = 0x0010

	ofnFileMustExist = 0x00001000
	ofnPathMustExist = 0x00000800

	mbIconWarning = 0x00000030
	mbIconError   = 0x00000010
	mbIconInfo    = 0x00000040
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassW      = user32.NewProc("RegisterClassW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetDlgItem          = user32.NewProc("GetDlgItem")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
	procOpenClipboard       = user32.NewProc("OpenClipboard")
	procEmptyClipboard      = user32.NewProc("EmptyClipboard")
	procSetClipboardData    = user32.NewProc("SetClipboardData")
	procCloseClipboard      = user32.NewProc("CloseClipboard")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procGlobalAlloc         = kernel32.NewProc("GlobalAlloc")
	procGlobalLock          = kernel32.NewProc("GlobalLock")
	procGlobalUnlock        = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory       = kernel32.NewProc("RtlMoveMemory")
	procGetOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")
	procSHBrowseForFolder   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree       = ole32.NewProc("CoTaskMemFree")
)

type wndclass struct {
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
}

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type point struct{ x, y int32 }

type openFileName struct {
	lStructSize     uint32
	hwndOwner       syscall.Handle
	hInstance       syscall.Handle
	lpstrFilter     *uint16
	lpstrCustomF    *uint16
	nMaxCustFilter  uint32
	nFilterIndex    uint32
	lpstrFile       *uint16
	nMaxFile        uint32
	lpstrFileTitle  *uint16
	nMaxFileTitle   uint32
	lpstrInitialDir *uint16
	lpstrTitle      *uint16
	flags           uint32
	nFileOffset     uint16
	nFileExtension  uint16
	lpstrDefExt     *uint16
	lCustData       uintptr
	lpfnHook        uintptr
	lpTemplateName  *uint16
}

type browseInfo struct {
	hwndOwner      syscall.Handle
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// gui 保存窗口句柄与运行中的 HTTP 服务。
type gui struct {
	hwnd  syscall.Handle
	hinst syscall.Handle

	mu  sync.Mutex
	srv *http.Server
}

var g *gui

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

func hinst() syscall.Handle {
	h, _, _ := procGetModuleHandleW.Call(0)
	return syscall.Handle(h)
}

func registerClass(h syscall.Handle) {
	wc := wndclass{
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     h,
		hIcon:         iconHandle(),
		hCursor:       cursorHandle(),
		lpszClassName: utf16(wndClassName),
	}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))
}

func iconHandle() syscall.Handle {
	h, _, _ := procLoadIconW.Call(0, IDIApplication)
	return syscall.Handle(h)
}

func cursorHandle() syscall.Handle {
	h, _, _ := procLoadCursorW.Call(0, IDCArrow)
	return syscall.Handle(h)
}

func wndProc(hwnd syscall.Handle, uMsg uint32, wParam, lParam uintptr) uintptr {
	switch uMsg {
	case wmCommand:
		g.onCommand(uint32(wParam & 0xffff))
		return 0
	case wmClose:
		g.stopServer()
		procPostQuitMessage.Call(0)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(uMsg), wParam, lParam)
	return r
}

func main() {
	for _, a := range os.Args[1:] {
		if a == "-selftest" {
			selftest = true
		}
	}
	h := hinst()
	registerClass(h)
	g = &gui{hinst: h}
	r, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16(wndClassName))),
		uintptr(unsafe.Pointer(utf16("Lan Share 直链分享"))),
		uintptr(wsCaption|wsSysMenu|wsMinimizeBox|wsThickFrame|wsMaximizeBox),
		cwUseDefault, cwUseDefault, 660, 520,
		0, 0, uintptr(h), 0)
	g.hwnd = syscall.Handle(r)
	if g.hwnd == 0 {
		msgbox(0, "窗口创建失败", "Lan Share", mbIconError)
		return
	}
	g.build()
	procShowWindow.Call(uintptr(g.hwnd), swShow)
	procUpdateWindow.Call(uintptr(g.hwnd))

	if selftest {
		go g.runSelfTest()
	}

	var m msg
	for {
		rc, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if rc == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// selftest 由 -selftest 命令行参数启用：自动启动服务并校验直链可达后退出。
var selftest bool

func (w *gui) runSelfTest() {
	root := "."
	if cwd, err := os.Getwd(); err == nil {
		root = cwd
	}
	// 探测本机 IP 推导的直链端口后请求
	// 由于 start() 内部自选端口且日志已含直链，直接读日志文本。
	// 先用固定端口避免并发杂音。
	time.Sleep(200 * time.Millisecond)

	// 启动服务：填当前目录 + 端口 18080
	w.setTextByID(IDEditPort, "18080")
	w.setTextByID(IDEditPath, root)
	w.start()

	// 从日志提取直链并做一次 HTTP 请求
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		txt := w.getTextByID(IDEditLog)
		for _, line := range strings.Split(txt, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "http://") {
				resp, err := http.Get(line)
				if err == nil && resp.StatusCode == 200 {
					resp.Body.Close()
					fmt.Println("SELFTEST OK: " + line)
					w.stopServer()
					os.Exit(0)
					return
				}
			}
		}
	}
	w.log("SELFTEST FAILED: no working URL")
	w.stopServer()
	os.Exit(1)
}

// build 创建全部子控件。
func (w *gui) build() {
	ff := defaultFont()
	w.label(12, 16, 64, 20, "分享路径:", ff)
	w.edit(IDEditPath, 84, 13, 396, 26, ff, esAutoHScroll)

	w.button(IDBtnFile, 486, 12, 70, 28, "选择文件", ff)
	w.button(IDBtnDir, 562, 12, 80, 28, "选择文件夹", ff)

	w.label(12, 54, 40, 20, "端口:", ff)
	w.edit(IDEditPort, 60, 51, 84, 26, ff, esAutoHScroll)
	w.label(160, 54, 84, 20, "Token(可选):", ff)
	w.edit(IDEditToken, 250, 51, 180, 26, ff, esAutoHScroll)
	w.button(IDBtnStart, 448, 51, 96, 28, "启动服务", ff)
	w.button(IDBtnCopy, 552, 51, 90, 28, "复制直链", ff)

	log := w.edit(IDEditLog, 12, 94, 630, 382, ff, esMultiline|esReadOnly|esWantReturn|wsVScroll|wsBorder)
	setText(log, "选择文件或文件夹，设置端口（默认 8080）与 Token（可选），点击「启动服务」。\n直链显示在下方，可一键复制。")
}

func defaultFont() syscall.Handle {
	h, _, _ := procGetStockObject.Call(defaultGuiFont)
	return syscall.Handle(h)
}

func (w *gui) label(x, y, cx, cy int, text string, f syscall.Handle) {
	w.child("STATIC", text, wsChild|wsVisible, x, y, cx, cy, 0, f)
}

func (w *gui) edit(id, x, y, cx, cy int, f syscall.Handle, style uint32) syscall.Handle {
	return w.child("EDIT", "", wsChild|wsVisible|wsBorder|style, x, y, cx, cy, uintptr(id), f)
}

func (w *gui) button(id, x, y, cx, cy int, text string, f syscall.Handle) {
	w.child("BUTTON", text, wsChild|wsVisible|wsTabsTop|bsPushButton, x, y, cx, cy, uintptr(id), f)
}

func (w *gui) child(class, text string, style uint32, x, y, cx, cy int, id uintptr, f syscall.Handle) syscall.Handle {
	h, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16(class))),
		uintptr(unsafe.Pointer(utf16(text))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(cx), uintptr(cy),
		uintptr(w.hwnd), id, uintptr(w.hinst), 0)
	procSendMessageW.Call(h, wmSetfont, 0, uintptr(f))
	return syscall.Handle(h)
}

func setText(h syscall.Handle, s string) {
	procSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(utf16(s))))
}

func getText(h syscall.Handle) string {
	buf := make([]uint16, 4096)
	n, _, _ := procGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

func (w *gui) ctrl(id uintptr) syscall.Handle {
	h, _, _ := procGetDlgItem.Call(uintptr(w.hwnd), id)
	return syscall.Handle(h)
}

func (w *gui) getTextByID(id uintptr) string {
	return getText(w.ctrl(id))
}

func (w *gui) setTextByID(id uintptr, s string) {
	setText(w.ctrl(id), s)
}

func (w *gui) onCommand(id uint32) {
	switch id {
	case IDBtnFile:
		w.pickFile()
	case IDBtnDir:
		w.pickDir()
	case IDBtnStart:
		w.start()
	case IDBtnCopy:
		w.copyLink()
	}
}

func (w *gui) pickFile() {
	var ofn openFileName
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	ofn.hwndOwner = w.hwnd
	ofn.lpstrFilter = utf16("所有文件 (*.*)\x00*.*\x00")
	buf := make([]uint16, maxPath)
	ofn.lpstrFile = &buf[0]
	ofn.nMaxFile = maxPath
	ofn.lpstrTitle = utf16("选择要分享的文件")
	ofn.flags = ofnFileMustExist | ofnPathMustExist
	if r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r != 0 {
		w.setTextByID(IDEditPath, syscall.UTF16ToString(buf))
	}
}

func (w *gui) pickDir() {
	disp := make([]uint16, maxPath)
	bi := browseInfo{
		hwndOwner:      w.hwnd,
		pszDisplayName: &disp[0],
		lpszTitle:      utf16("选择要分享的文件夹"),
		ulFlags:        bifReturnOnlyFSDirs | bifNewDialogStyle | bifEditBox,
	}
	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return
	}
	buf := make([]uint16, maxPath)
	if r, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&buf[0]))); r != 0 {
		w.setTextByID(IDEditPath, syscall.UTF16ToString(buf))
	}
	procCoTaskMemFree.Call(pidl)
}

func (w *gui) start() {
	w.mu.Lock()
	if w.srv != nil {
		w.mu.Unlock()
		msgbox(w.hwnd, "服务已在运行，请先关闭窗口或停止。", "Lan Share", mbIconInfo)
		return
	}
	w.mu.Unlock()

	path := strings.TrimSpace(w.getTextByID(IDEditPath))
	if path == "" {
		path = "."
	}
	portStr := strings.TrimSpace(w.getTextByID(IDEditPort))
	port := 8080
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			w.log("端口无效: %s", portStr)
			return
		}
		port = p
	}
	token := strings.TrimSpace(w.getTextByID(IDEditToken))

	abs, err := filepath.Abs(path)
	if err != nil {
		w.log("路径解析失败: %v", err)
		return
	}
	fi, err := os.Stat(abs)
	if err != nil {
		w.log("无法访问路径: %v", err)
		return
	}
	port, err = server.NextPort(port, 10)
	if err != nil {
		w.log("端口不可用: %v", err)
		return
	}

	root := abs
	rel := "/"
	if !fi.IsDir() {
		root = filepath.Dir(abs)
		rel = "/" + filepath.Base(abs)
	}

	s := server.New(root)
	if token != "" {
		s.SetSecret(token)
	}

	ips := ipaddr.ListLocalIPs()
	var urls []string
	for _, ip := range ips {
		urls = append(urls, s.LinkURL(ip, port, rel, time.Time{}))
	}

	srv := &http.Server{Addr: "0.0.0.0:" + strconv.Itoa(port), Handler: s}
	w.mu.Lock()
	w.srv = srv
	w.mu.Unlock()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			w.log("HTTP 服务异常: %v", err)
		}
	}()

	var b strings.Builder
	if fi.IsDir() {
		b.WriteString("分享目录: " + abs + "\n")
	} else {
		b.WriteString("分享文件: " + abs + " (" + server.HumanSize(fi.Size()) + ")\n")
	}
	b.WriteString("端口: " + strconv.Itoa(port) + "\n")
	if token == "" {
		b.WriteString("访问保护: 无\n")
	} else {
		b.WriteString("访问保护: 开启\n")
	}
	b.WriteString("直链:\n")
	for _, u := range urls {
		b.WriteString(u + "\n")
	}
	w.setTextByID(IDEditLog, b.String())
}

func (w *gui) stopServer() {
	w.mu.Lock()
	srv := w.srv
	w.srv = nil
	w.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func (w *gui) copyLink() {
	url := firstURL(w.getTextByID(IDEditLog))
	if url == "" {
		msgbox(w.hwnd, "没有可复制的直链。", "Lan Share", mbIconInfo)
		return
	}
	setClipboard(url)
	w.log("%s", "已复制: "+url)
}

// firstURL 从文本中提取第一个 http(s) 直链。
func firstURL(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}

func (w *gui) log(format string, args ...interface{}) {
	prev := w.getTextByID(IDEditLog)
	if !strings.Contains(format, "%") {
		format = "%s"
	}
	w.setTextByID(IDEditLog, prev+"\n"+fmt.Sprintf(format, args...))
}

func setClipboard(text string) {
	procOpenClipboard.Call(0)
	procEmptyClipboard.Call(0)
	data := syscall.StringToUTF16(text)
	h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(2*len(data)))
	if h != 0 {
		p, _, _ := procGlobalLock.Call(h)
		if p != 0 {
			procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), uintptr(2*len(data)))
			procGlobalUnlock.Call(h)
			procSetClipboardData.Call(cfUnicodeText, h)
		}
	}
	procCloseClipboard.Call()
}

func msgbox(hwnd syscall.Handle, text, title string, style uint32) {
	procMessageBoxW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(utf16(text))), uintptr(unsafe.Pointer(utf16(title))), uintptr(style))
}
