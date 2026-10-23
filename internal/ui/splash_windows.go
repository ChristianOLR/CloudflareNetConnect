//go:build windows

package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type SplashWindow struct {
	authURL      string
	hWnd         windows.Handle
	hLogoIcon    windows.Handle
	mu           sync.Mutex
	done         chan struct{}
	closeOnce    sync.Once
	animTick     int
	copiedState  bool
	successState bool
	statusText   string
}

func NewSplashWindow(authURL string) *SplashWindow {
	return &SplashWindow{
		authURL:    authURL,
		done:       make(chan struct{}),
		statusText: "Esperando identificación...",
	}
}

func (s *SplashWindow) Show() {
	go s.runNativeFrameless()
}

func (s *SplashWindow) SetSuccess(accName string) {
	s.mu.Lock()
	s.successState = true
	s.statusText = "✅ ¡Cuenta " + accName + " vinculada!"
	h := s.hWnd
	s.mu.Unlock()
	if h != 0 {
		windows.NewLazySystemDLL("user32.dll").NewProc("InvalidateRect").Call(uintptr(h), 0, 0)
	}
}

func (s *SplashWindow) Close() {
	s.mu.Lock()
	h := s.hWnd
	s.mu.Unlock()
	if h != 0 {
		windows.NewLazySystemDLL("user32.dll").NewProc("PostMessageW").Call(uintptr(h), 0x0010, 0, 0)
	}
	s.closeOnce.Do(func() { close(s.done) })
}

func (s *SplashWindow) Wait() {
	<-s.done
}

func (s *SplashWindow) runNativeFrameless() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer s.closeOnce.Do(func() { close(s.done) })

	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	gdi := windows.NewLazySystemDLL("gdi32.dll")

	hInst, _, _ := k32.NewProc("GetModuleHandleW").Call(0)
	clsName, _ := windows.UTF16PtrFromString(fmt.Sprintf("CFSplashCard_%d", time.Now().UnixNano()))

	type wndClassEx struct {
		Size, Style                        uint32
		WndProc                            uintptr
		ClsExtra, WndExtra                 int32
		Instance, Icon, Cursor, Background windows.Handle
		MenuName, ClassName                *uint16
		IconSm                             windows.Handle
	}

	hCursor, _, _ := u32.NewProc("LoadCursorW").Call(0, 32512)

	wcex := &wndClassEx{
		Size: 80,
		WndProc: windows.NewCallback(func(hWnd windows.Handle, msg uint32, wp, lp uintptr) uintptr {
			switch msg {
			case 0x0014: // WM_ERASEBKGND
				return 1
			case 0x000F: // WM_PAINT
				s.paintCard(hWnd)
				return 0
			case 0x0113: // WM_TIMER
				s.mu.Lock()
				s.animTick++
				if !s.successState {
					dots := (s.animTick / 10) % 4
					dStr := ""
					for i := 0; i < dots; i++ {
						dStr += "."
					}
					s.statusText = "Esperando identificación" + dStr
				}
				s.mu.Unlock()
				u32.NewProc("InvalidateRect").Call(uintptr(hWnd), 0, 0)
				return 0
			case 0x0201: // WM_LBUTTONDOWN
				x := int32(lp & 0xFFFF)
				y := int32((lp >> 16) & 0xFFFF)
				// Botón 1: Copiar Enlace [32..348, 224..268]
				if x >= 32 && x <= 348 && y >= 224 && y <= 268 {
					CopyToClipboard(s.authURL)
					s.mu.Lock()
					s.copiedState = true
					s.mu.Unlock()
					u32.NewProc("InvalidateRect").Call(uintptr(hWnd), 0, 0)
					go func() {
						time.Sleep(3 * time.Second)
						s.mu.Lock()
						s.copiedState = false
						s.mu.Unlock()
					}()
					return 0
				}
				// Botón 2: Abrir en Navegador [32..348, 280..322]
				if x >= 32 && x <= 348 && y >= 280 && y <= 322 {
					_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", s.authURL).Start()
					return 0
				}
				// Botón 3: Cancelar [125..255, 340..374]
				if x >= 125 && x <= 255 && y >= 340 && y <= 374 {
					u32.NewProc("DestroyWindow").Call(uintptr(hWnd))
					return 0
				}
				// Arrastrar ventana desde cualquier punto libre
				u32.NewProc("ReleaseCapture").Call()
				u32.NewProc("SendMessageW").Call(uintptr(hWnd), 0x00A1, 2, 0)
				return 0
			case 0x0002: // WM_DESTROY
				u32.NewProc("KillTimer").Call(uintptr(hWnd), 1)
				u32.NewProc("PostQuitMessage").Call(0)
				return 0
			}
			r, _, _ := u32.NewProc("DefWindowProcW").Call(uintptr(hWnd), uintptr(msg), wp, lp)
			return r
		}),
		Instance:  windows.Handle(hInst),
		Cursor:    windows.Handle(hCursor),
		ClassName: clsName,
	}
	u32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(wcex)))

	w, h := uintptr(380), uintptr(410)
	scrW, _, _ := u32.NewProc("GetSystemMetrics").Call(0)
	scrH, _, _ := u32.NewProc("GetSystemMetrics").Call(1)
	posX := (scrW - w) / 2
	posY := (scrH - h) / 2

	title, _ := windows.UTF16PtrFromString("CloudflareNetConnect")
	hWin, _, _ := u32.NewProc("CreateWindowExW").Call(
		0x00000008,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(title)),
		0x80000000|0x10000000,
		posX, posY, w, h,
		0, 0, hInst, 0,
	)

	hRgn, _, _ := gdi.NewProc("CreateRoundRectRgn").Call(0, 0, w+1, h+1, 24, 24)
	u32.NewProc("SetWindowRgn").Call(hWin, hRgn, 1)

	s.mu.Lock()
	s.hWnd = windows.Handle(hWin)
	s.mu.Unlock()

	u32.NewProc("SetTimer").Call(hWin, 1, 45, 0)

	type msg struct {
		HWnd    windows.Handle
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      struct{ X, Y int32 }
	}
	var m msg
	for {
		ret, _, _ := u32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		u32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		u32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}
}

func CopyToClipboard(text string) {
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	r, _, _ := u32.NewProc("OpenClipboard").Call(0)
	if r == 0 {
		return
	}
	defer u32.NewProc("CloseClipboard").Call()
	u32.NewProc("EmptyClipboard").Call()

	utf16, _ := windows.UTF16FromString(text)
	hMem, _, _ := k32.NewProc("GlobalAlloc").Call(0x0042, uintptr(len(utf16)*2))
	if hMem == 0 {
		return
	}
	ptr, _, _ := k32.NewProc("GlobalLock").Call(hMem)
	if ptr != 0 {
		dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
		copy(dst, utf16)
		k32.NewProc("GlobalUnlock").Call(hMem)
		u32.NewProc("SetClipboardData").Call(13, hMem)
	}
}
