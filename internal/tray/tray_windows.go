//go:build windows

package tray

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"unsafe"

	"github.com/olrsoft/cloudflarenetconnect/assets"
	"github.com/olrsoft/cloudflarenetconnect/internal/config"
	"github.com/olrsoft/cloudflarenetconnect/internal/tunnel"
	"golang.org/x/sys/windows"
)

const (
	wmAppTray     = 0x8000 + 1
	idMaster      = 1001
	idAssociate   = 1002
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
	orch       *tunnel.Orchestrator
	cfg        *config.AppConfig
	configPath string
	mu         sync.Mutex
	hWnd       windows.Handle
	hIcon      windows.Handle
	nid        NOTIFYICONDATAW
}

func NewTrayApp(orch *tunnel.Orchestrator, cfg *config.AppConfig, configPath string) *TrayApp {
	return &TrayApp{orch: orch, cfg: cfg, configPath: configPath}
}

func (t *TrayApp) Run() {
	log.Println("[TRAY] Iniciando bucle de bandeja...")
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
		Size: 80,
		WndProc: windows.NewCallback(func(hWnd windows.Handle, msg uint32, wp, lp uintptr) uintptr {
			if msg == wmAppTray && (lp == 0x0205 || lp == 0x0202) {
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
		UFlags:           7,
		UCallbackMessage: wmAppTray,
		HIcon:            t.hIcon,
	}
	tip, _ := windows.UTF16FromString("CloudflareNetConnect - Multi-Cuenta (ON)")
	copy(t.nid.SzTip[:], tip)

	res, _, err := pShellNotifyIcon.Call(0, uintptr(unsafe.Pointer(&t.nid)))
	log.Printf("[TRAY] ShellNotifyIcon NIM_ADD res: %d, err: %v, hWnd: %x\n", res, err, t.hWnd)

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
		ret, _, err := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			log.Printf("[TRAY] GetMessage salió con ret: %d, err: %v\n", ret, err)
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

	appendStr(0x0001, 0, "☁️ CloudflareNetConnect")
	appendStr(0x0800, 0, "") // Separator

	toggleFlag := uintptr(0x0000)
	if t.cfg.MasterToggle {
		toggleFlag = 0x0008 // MF_CHECKED
	}
	appendStr(toggleFlag, idMaster, "Interruptor Maestro (ON / OFF)")
	appendStr(0x0800, 0, "") // Separator

	// Listado de cuentas asociadas
	t.mu.Lock()
	accountMap := make(map[uintptr]int)
	for i, acc := range t.cfg.Accounts {
		flag := uintptr(0x0000)
		if acc.Enabled {
			flag = 0x0008 // MF_CHECKED
		}
		accID := uintptr(idAccountBase + i)
		accountMap[accID] = i
		appendStr(flag, accID, fmt.Sprintf("✓ %s", acc.Name))
	}
	t.mu.Unlock()

	appendStr(0x0800, 0, "") // Separator
	appendStr(0x0000, idAssociate, "➕ Asociar nueva cuenta...")
	appendStr(0x0800, 0, "") // Separator
	appendStr(0x0000, idQuit, "❌ Salir")

	var pt struct{ X, Y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(hWnd))

	cmd, _, _ := pTrackPopupMenu.Call(uintptr(hPopup), 0x0100|0x0002, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hWnd), 0)

	if idx, ok := accountMap[cmd]; ok {
		t.mu.Lock()
		t.cfg.Accounts[idx].Enabled = !t.cfg.Accounts[idx].Enabled
		_ = config.SaveConfig(t.configPath, t.cfg)
		t.mu.Unlock()
		log.Printf("[TRAY] Conmutada cuenta '%s': %v\n", t.cfg.Accounts[idx].Name, t.cfg.Accounts[idx].Enabled)
		return
	}

	switch cmd {
	case idMaster:
		t.cfg.MasterToggle = !t.cfg.MasterToggle
		t.orch.SetMasterToggle(t.cfg.MasterToggle)
		log.Printf("[TRAY] Conmutador Maestro: %v\n", t.cfg.MasterToggle)
	case idAssociate:
		t.StartAssociateFlow()
	case idQuit:
		log.Println("[TRAY] Cierre ordenado solicitado.")
		_ = t.orch.Stop()
		pPostQuitMessage.Call(0)
	}
}
