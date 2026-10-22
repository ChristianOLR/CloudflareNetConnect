//go:build windows

package ui

import (
	"os/exec"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type SplashWindow struct {
	authURL string
	hWnd    windows.Handle
	mu      sync.Mutex
	done    chan struct{}
}

func NewSplashWindow(authURL string) *SplashWindow {
	return &SplashWindow{
		authURL: authURL,
		done:    make(chan struct{}),
	}
}

func (s *SplashWindow) Show() {
	go s.runWindow()
}

func (s *SplashWindow) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hWnd != 0 {
		u32 := windows.NewLazySystemDLL("user32.dll")
		pPostMessage := u32.NewProc("PostMessageW")
		pPostMessage.Call(uintptr(s.hWnd), 0x0010, 0, 0) // WM_CLOSE
	}
}

func (s *SplashWindow) Wait() {
	<-s.done
}

func (s *SplashWindow) runWindow() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(s.done)

	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	gdi := windows.NewLazySystemDLL("gdi32.dll")

	pGetModuleHandle := k32.NewProc("GetModuleHandleW")
	pRegisterClass := u32.NewProc("RegisterClassExW")
	pCreateWindowEx := u32.NewProc("CreateWindowExW")
	pDefWindowProc := u32.NewProc("DefWindowProcW")
	pGetMessage := u32.NewProc("GetMessageW")
	pTranslateMessage := u32.NewProc("TranslateMessage")
	pDispatchMessage := u32.NewProc("DispatchMessageW")
	pDestroyWindow := u32.NewProc("DestroyWindow")
	pSetTimer := u32.NewProc("SetTimer")
	pKillTimer := u32.NewProc("KillTimer")
	pSetWindowText := u32.NewProc("SetWindowTextW")
	pCreateSolidBrush := gdi.NewProc("CreateSolidBrush")

	hInst, _, _ := pGetModuleHandle.Call(0)
	hBg, _, _ := pCreateSolidBrush.Call(0x002A170F) // RGB(15, 23, 42)

	clsName, _ := windows.UTF16PtrFromString("CloudflareSplashClass")
	type wndClassEx struct {
		Size, Style                        uint32
		WndProc                            uintptr
		ClsExtra, WndExtra                 int32
		Instance, Icon, Cursor, Background windows.Handle
		MenuName, ClassName                *uint16
		IconSm                             windows.Handle
	}

	var hStatusLabel windows.Handle
	var hCopyBtn windows.Handle
	dots := 0

	wcex := &wndClassEx{
		Size: 80,
		WndProc: windows.NewCallback(func(hWnd windows.Handle, msg uint32, wp, lp uintptr) uintptr {
			switch msg {
			case 0x0111: // WM_COMMAND
				cmdID := wp & 0xFFFF
				if cmdID == 201 { // Copiar
					s.copyToClipboard()
					txt, _ := windows.UTF16PtrFromString("✅ ¡Enlace Copiado!")
					pSetWindowText.Call(uintptr(hCopyBtn), uintptr(unsafe.Pointer(txt)))
				} else if cmdID == 202 { // Abrir
					_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", s.authURL).Start()
				} else if cmdID == 203 { // Cancelar
					pDestroyWindow.Call(uintptr(hWnd))
				}
				return 0
			case 0x0113: // WM_TIMER
				dots = (dots + 1) % 4
				dStr := ""
				for i := 0; i < dots; i++ {
					dStr += "."
				}
				t, _ := windows.UTF16PtrFromString("Esperando identificación" + dStr)
				pSetWindowText.Call(uintptr(hStatusLabel), uintptr(unsafe.Pointer(t)))
				return 0
			case 0x0002: // WM_DESTROY
				pKillTimer.Call(uintptr(hWnd), 1)
				u32.NewProc("PostQuitMessage").Call(0)
				return 0
			}
			r, _, _ := pDefWindowProc.Call(uintptr(hWnd), uintptr(msg), wp, lp)
			return r
		}),
		Instance:   windows.Handle(hInst),
		Background: windows.Handle(hBg),
		ClassName:  clsName,
	}
	pRegisterClass.Call(uintptr(unsafe.Pointer(wcex)))

	title, _ := windows.UTF16PtrFromString("CloudflareNetConnect - Asociar Nueva Cuenta")
	hWin, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(clsName)), uintptr(unsafe.Pointer(title)), 0x00CA0000|0x10000000, 400, 250, 420, 280, 0, 0, hInst, 0)
	s.mu.Lock()
	s.hWnd = windows.Handle(hWin)
	s.mu.Unlock()

	btnCls, _ := windows.UTF16PtrFromString("BUTTON")
	staticCls, _ := windows.UTF16PtrFromString("STATIC")

	hTitle, _ := windows.UTF16PtrFromString("☁️ Conectar Cuenta Cloudflare")
	pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(staticCls)), uintptr(unsafe.Pointer(hTitle)), 0x50000000|0x01, 20, 20, 360, 25, uintptr(s.hWnd), 0, hInst, 0)

	hInitStatus, _ := windows.UTF16PtrFromString("Esperando identificación en el navegador...")
	hLbl, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(staticCls)), uintptr(unsafe.Pointer(hInitStatus)), 0x50000000|0x01, 20, 55, 360, 25, uintptr(s.hWnd), 0, hInst, 0)
	hStatusLabel = windows.Handle(hLbl)

	hCopyTxt, _ := windows.UTF16PtrFromString("📋 Copiar Enlace")
	hCBtn, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(btnCls)), uintptr(unsafe.Pointer(hCopyTxt)), 0x50000000|0x0001, 35, 100, 160, 35, uintptr(s.hWnd), 201, hInst, 0)
	hCopyBtn = windows.Handle(hCBtn)

	hOpenTxt, _ := windows.UTF16PtrFromString("🌐 Abrir Navegador")
	pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(btnCls)), uintptr(unsafe.Pointer(hOpenTxt)), 0x50000000, 210, 100, 160, 35, uintptr(s.hWnd), 202, hInst, 0)

	hCancelTxt, _ := windows.UTF16PtrFromString("Cancelar")
	pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(btnCls)), uintptr(unsafe.Pointer(hCancelTxt)), 0x50000000, 135, 160, 140, 30, uintptr(s.hWnd), 203, hInst, 0)

	pSetTimer.Call(uintptr(s.hWnd), 1, 400, 0)

	// Abrir automáticamente el navegador al inicio
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", s.authURL).Start()
	}()

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
		ret, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (s *SplashWindow) copyToClipboard() {
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	pOpenClipboard := u32.NewProc("OpenClipboard")
	pEmptyClipboard := u32.NewProc("EmptyClipboard")
	pSetClipboardData := u32.NewProc("SetClipboardData")
	pCloseClipboard := u32.NewProc("CloseClipboard")
	pGlobalAlloc := k32.NewProc("GlobalAlloc")
	pGlobalLock := k32.NewProc("GlobalLock")
	pGlobalUnlock := k32.NewProc("GlobalUnlock")

	pOpenClipboard.Call(0)
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()

	utf16, _ := windows.UTF16FromString(s.authURL)
	bytesCount := len(utf16) * 2

	hMem, _, _ := pGlobalAlloc.Call(0x0042, uintptr(bytesCount)) // GMEM_MOVEABLE | GMEM_ZEROINIT
	ptr, _, _ := pGlobalLock.Call(hMem)
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
	copy(dst, utf16)
	pGlobalUnlock.Call(hMem)

	pSetClipboardData.Call(13, hMem) // CF_UNICODETEXT = 13
}
