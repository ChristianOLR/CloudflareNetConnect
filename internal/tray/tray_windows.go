//go:build windows

package tray

import (
	"log"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/assets"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/tunnel"
	"golang.org/x/sys/windows"
)

const (
	wmAppTray       = 0x8000 + 1
	idHeader        = 1000
	idMaster        = 1001
	idAssociate     = 1002
	idQuit          = 1003
	idStartupToggle = 1010
	idSvcInstall    = 1011
	idSvcStart      = 1012
	idSvcStop       = 1013
	idSvcUninstall  = 1014
	idAccToggleBase  = 2000
	idAccRemoveBase  = 3000
	idAccRefreshBase = 4000
)

type TrayApp struct {
	orch       *tunnel.Orchestrator
	cfg        *config.AppConfig
	configPath string
	mu         sync.Mutex
	hWnd       windows.Handle
	hIcon      windows.Handle
	hDotGreen  windows.Handle
	hDotRed    windows.Handle
	nid        NOTIFYICONDATAW
}

func NewTrayApp(orch *tunnel.Orchestrator, cfg *config.AppConfig, configPath string) *TrayApp {
	return &TrayApp{orch: orch, cfg: cfg, configPath: configPath}
}

func (t *TrayApp) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u32 := windows.NewLazySystemDLL("user32.dll")
	s32 := windows.NewLazySystemDLL("shell32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")

	// Vincular el hilo al escritorio interactivo 'Default' de Windows
	attachDefaultDesktop(u32)

	hInst, _, _ := k32.NewProc("GetModuleHandleW").Call(0)
	t.hDotGreen = createColorDotBitmap(true)
	t.hDotRed = createColorDotBitmap(false)
	t.hIcon = t.createHIcon(t.isEffectivelyActive())

	clsName, _ := windows.UTF16PtrFromString("CloudflareNetConnectTray")
	type wndClassEx struct {
		Size, Style                        uint32
		WndProc                            uintptr
		ClsExtra, WndExtra                 int32
		Instance, Icon, Cursor, Background windows.Handle
		MenuName, ClassName                *uint16
		IconSm                             windows.Handle
	}
	wcex := &wndClassEx{
		Size: 80,
		WndProc: windows.NewCallback(func(hWnd windows.Handle, msg uint32, wp, lp uintptr) uintptr {
			if msg == wmAppTray && (lp == 0x0205 || lp == 0x0202) {
				t.showMenu(hWnd)
				return 0
			}
			r, _, _ := u32.NewProc("DefWindowProcW").Call(uintptr(hWnd), uintptr(msg), wp, lp)
			return r
		}),
		Instance:  windows.Handle(hInst),
		Icon:      t.hIcon,
		ClassName: clsName,
	}
	u32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(wcex)))

	wName, _ := windows.UTF16PtrFromString("CloudflareNetConnectHidden")
	hWin, _, errWin := u32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(clsName)), uintptr(unsafe.Pointer(wName)), 0, 0, 0, 0, 0, 0, 0, hInst, 0)
	t.hWnd = windows.Handle(hWin)
	log.Printf("[TRAY-DEBUG] CreateWindowExW hWin=%v, errWin=%v\n", hWin, errWin)

	t.nid = NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(t.nid)),
		HWnd:             t.hWnd,
		UID:              100,
		UFlags:           7,
		UCallbackMessage: wmAppTray,
		HIcon:            t.hIcon,
	}

	// Limpiar icono previo si existiera (reinicio previo o crash)
	s32.NewProc("Shell_NotifyIconW").Call(2, uintptr(unsafe.Pointer(&t.nid)))

	var rAdd uintptr
	var errAdd error
	for attempt := 1; attempt <= 5; attempt++ {
		rAdd, _, errAdd = s32.NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&t.nid)))
		if rAdd != 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Printf("[TRAY] Shell_NotifyIconW NIM_ADD ret=%v, err=%v, cbSize=%d\n", rAdd, errAdd, t.nid.CbSize)
	t.refreshTrayState()
	go t.AutoSyncMissingRoutes()
	log.Println("[TRAY] Icono de Cloudflare anclado en la barra de tareas.")

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
		ret, _, errGet := u32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			log.Printf("[TRAY-DEBUG] GetMessageW ret=%d, err=%v\n", ret, errGet)
			break
		}
		u32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		u32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}

	s32.NewProc("Shell_NotifyIconW").Call(2, uintptr(unsafe.Pointer(&t.nid)))
	u32.NewProc("DestroyWindow").Call(uintptr(t.hWnd))
}

func attachDefaultDesktop(u32 *windows.LazyDLL) {
	defDeskName, _ := windows.UTF16PtrFromString("Default")
	hDefDesk, _, _ := u32.NewProc("OpenDesktopW").Call(
		uintptr(unsafe.Pointer(defDeskName)),
		0,
		0,
		0x01FF,
	)
	if hDefDesk != 0 {
		u32.NewProc("SetThreadDesktop").Call(hDefDesk)
	}
}

func (t *TrayApp) createHIcon(active bool) windows.Handle {
	u32 := windows.NewLazySystemDLL("user32.dll")
	pngBytes := assets.CloudflareGreenPNG
	if !active {
		pngBytes = assets.CloudflareRedPNG
	}
	hIcon, _, _ := u32.NewProc("CreateIconFromResourceEx").Call(uintptr(unsafe.Pointer(&pngBytes[0])), uintptr(len(pngBytes)), 1, 0x00030000, 32, 32, 0)
	return windows.Handle(hIcon)
}
