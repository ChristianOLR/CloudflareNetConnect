//go:build windows

package tray

import (
	"log"
	"runtime"
	"sync"
	"unsafe"

	"github.com/olrsoft/cloudflarenetconnect/assets"
	"github.com/olrsoft/cloudflarenetconnect/internal/cloudflare"
	"github.com/olrsoft/cloudflarenetconnect/internal/config"
	"github.com/olrsoft/cloudflarenetconnect/internal/tunnel"
	"golang.org/x/sys/windows"
)

const (
	wmAppTray     = 0x8000 + 1
	idMaster      = 1001
	idRefresh     = 1002
	idQuit        = 1003
	idAccountBase = 2000
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	_                uint32
	HWnd             windows.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	_                uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutOrVer    uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}

type TrayApp struct {
	orch  *tunnel.Orchestrator
	cfg   *config.AppConfig
	mu    sync.Mutex
	hWnd  windows.Handle
	hIcon windows.Handle
	nid   NOTIFYICONDATAW
}

func NewTrayApp(orch *tunnel.Orchestrator, cfg *config.AppConfig) *TrayApp {
	return &TrayApp{orch: orch, cfg: cfg}
}

func (t *TrayApp) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u32 := windows.NewLazySystemDLL("user32.dll")
	s32 := windows.NewLazySystemDLL("shell32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")

	pGetModuleHandle := k32.NewProc("GetModuleHandleW")
	pCreateIconFromResourceEx := u32.NewProc("CreateIconFromResourceEx")
	pRegisterClass := u32.NewProc("RegisterClassExW")
	pCreateWindowEx := u32.NewProc("CreateWindowExW")
	pDefWindowProc := u32.NewProc("DefWindowProcW")
	pShellNotifyIcon := s32.NewProc("Shell_NotifyIconW")
	pGetMessage := u32.NewProc("GetMessageW")
	pTranslateMessage := u32.NewProc("TranslateMessage")
	pDispatchMessage := u32.NewProc("DispatchMessageW")
	pDestroyWindow := u32.NewProc("DestroyWindow")

	hInst, _, _ := pGetModuleHandle.Call(0)
	png := assets.CloudflarePNG
	hIcon, _, _ := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, 0x00030000, 32, 32, 0)
	t.hIcon = windows.Handle(hIcon)

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
		Size:     uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:  windows.NewCallback(func(hWnd windows.Handle, msg uint32, wp, lp uintptr) uintptr {
			if msg == wmAppTray && (lp == 0x0205 || lp == 0x0202) { // WM_RBUTTONUP o WM_LBUTTONUP
				t.showMenu(hWnd)
				return 0
			}
			r, _, _ := pDefWindowProc.Call(uintptr(hWnd), uintptr(msg), wp, lp)
			return r
		}),
		Instance:  windows.Handle(hInst),
		Icon:      t.hIcon,
		ClassName: clsName,
	}
	pRegisterClass.Call(uintptr(unsafe.Pointer(wcex)))

	wName, _ := windows.UTF16PtrFromString("CloudflareNetConnectHidden")
	hWin, _, _ := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(clsName)), uintptr(unsafe.Pointer(wName)), 0x00CF0000, 0, 0, 0, 0, 0, 0, hInst, 0)
	t.hWnd = windows.Handle(hWin)

	t.nid = NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(t.nid)),
		HWnd:             t.hWnd,
		UID:              100,
		UFlags:           7, // NIF_MESSAGE | NIF_ICON | NIF_TIP
		UCallbackMessage: wmAppTray,
		HIcon:            t.hIcon,
	}
	tip, _ := windows.UTF16FromString("CloudflareNetConnect - Multi-Cuenta (ON)")
	copy(t.nid.SzTip[:], tip)

	pShellNotifyIcon.Call(0, uintptr(unsafe.Pointer(&t.nid)))
	log.Println("[TRAY] Icono oficial de Cloudflare anclado en la barra de tareas.")

	// Message loop
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

	pShellNotifyIcon.Call(2, uintptr(unsafe.Pointer(&t.nid)))
	pDestroyWindow.Call(uintptr(t.hWnd))
}

func (t *TrayApp) showMenu(hWnd windows.Handle) {
	u32 := windows.NewLazySystemDLL("user32.dll")
	pCreatePopupMenu := u32.NewProc("CreatePopupMenu")
	pAppendMenu := u32.NewProc("AppendMenuW")
	pTrackPopupMenu := u32.NewProc("TrackPopupMenu")
	pGetCursorPos := u32.NewProc("GetCursorPos")
	pSetForegroundWindow := u32.NewProc("SetForegroundWindow")
	pPostQuitMessage := u32.NewProc("PostQuitMessage")

	hMenu, _, _ := pCreatePopupMenu.Call()
	hPopup := windows.Handle(hMenu)

	appendStr := func(flags uintptr, id uintptr, text string) {
		ptr, _ := windows.UTF16PtrFromString(text)
		pAppendMenu.Call(uintptr(hPopup), flags, id, uintptr(unsafe.Pointer(ptr)))
	}

	appendStr(0x0001, 0, "☁️ CloudflareNetConnect (Activo)")
	appendStr(0x0800, 0, "") // Separator

	toggleFlag := uintptr(0x0000)
	if t.cfg.MasterToggle {
		toggleFlag = 0x0008 // MF_CHECKED
	}
	appendStr(toggleFlag, idMaster, "Interruptor Maestro (ON / OFF)")
	appendStr(0x0000, idRefresh, "🔄 Auto-Descubrir Cuentas (Cloudflare API)")
	appendStr(0x0800, 0, "") // Separator
	appendStr(0x0000, idQuit, "❌ Salir")

	var pt struct{ X, Y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(hWnd))

	cmd, _, _ := pTrackPopupMenu.Call(uintptr(hPopup), 0x0100|0x0002, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hWnd), 0)

	switch cmd {
	case idMaster:
		t.cfg.MasterToggle = !t.cfg.MasterToggle
		t.orch.SetMasterToggle(t.cfg.MasterToggle)
		log.Printf("[TRAY] Conmutador Maestro: %v\n", t.cfg.MasterToggle)
	case idRefresh:
		log.Println("[TRAY] Consultando organizaciones Cloudflare...")
		go t.discoverAccounts()
	case idQuit:
		log.Println("[TRAY] Cierre ordenado solicitado.")
		_ = t.orch.Stop()
		pPostQuitMessage.Call(0)
	}
}

func (t *TrayApp) discoverAccounts() {
	for _, acc := range t.cfg.Accounts {
		if acc.APIToken != "" {
			client := cloudflare.NewClient(acc.APIToken)
			accounts, err := client.ListAccounts()
			if err != nil {
				log.Printf("[API-DISCOVERY] Error %s: %v\n", acc.ID, err)
				continue
			}
			for _, remoteAcc := range accounts {
				log.Printf("[API-DISCOVERY] Cuenta remota detectada: %s (%s)\n", remoteAcc.Name, remoteAcc.ID)
			}
		}
	}
}
