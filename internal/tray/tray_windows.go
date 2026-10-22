//go:build windows

package tray

import (
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
	wmAppTray       = 0x8000 + 1
	idMaster        = 1001
	idAssociateArgo = 1002
	idQuit          = 1003
	idAssocOLR      = 1004
	idAssocHispa    = 1005
	idAccToggleBase = 2000
	idAccRemoveBase = 3000
)

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
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	u32 := windows.NewLazySystemDLL("user32.dll")
	s32 := windows.NewLazySystemDLL("shell32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")

	pGetModuleHandle := k32.NewProc("GetModuleHandleW")
	pRegisterClass := u32.NewProc("RegisterClassExW")
	pCreateWindowEx := u32.NewProc("CreateWindowExW")
	pDefWindowProc := u32.NewProc("DefWindowProcW")
	pShellNotifyIcon := s32.NewProc("Shell_NotifyIconW")
	pGetMessage := u32.NewProc("GetMessageW")
	pTranslateMessage := u32.NewProc("TranslateMessage")
	pDispatchMessage := u32.NewProc("DispatchMessageW")
	pDestroyWindow := u32.NewProc("DestroyWindow")

	hInst, _, _ := pGetModuleHandle.Call(0)
	t.hIcon = t.createHIcon(t.cfg.MasterToggle)

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
		UFlags:           7, // NIF_MESSAGE | NIF_ICON | NIF_TIP
		UCallbackMessage: wmAppTray,
		HIcon:            t.hIcon,
	}
	tipText := "CloudflareNetConnect - Activo (ON)"
	if !t.cfg.MasterToggle {
		tipText = "CloudflareNetConnect - En Reposo (OFF)"
	}
	tip, _ := windows.UTF16FromString(tipText)
	copy(t.nid.SzTip[:], tip)

	pShellNotifyIcon.Call(0, uintptr(unsafe.Pointer(&t.nid)))
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

func (t *TrayApp) updateTrayIcon(active bool) {
	s32 := windows.NewLazySystemDLL("shell32.dll")
	pShellNotifyIcon := s32.NewProc("Shell_NotifyIconW")

	t.hIcon = t.createHIcon(active)
	t.nid.HIcon = t.hIcon
	tipText := "CloudflareNetConnect - Activo (ON)"
	if !active {
		tipText = "CloudflareNetConnect - En Reposo (OFF)"
	}
	tip, _ := windows.UTF16FromString(tipText)
	copy(t.nid.SzTip[:], tip)

	pShellNotifyIcon.Call(1, uintptr(unsafe.Pointer(&t.nid))) // NIM_MODIFY = 1
}

func (t *TrayApp) createHIcon(active bool) windows.Handle {
	u32 := windows.NewLazySystemDLL("user32.dll")
	pCreateIconFromResourceEx := u32.NewProc("CreateIconFromResourceEx")

	pngBytes := assets.CloudflareGreenPNG
	if !active {
		pngBytes = assets.CloudflareRedPNG
	}
	hIcon, _, _ := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&pngBytes[0])), uintptr(len(pngBytes)), 1, 0x00030000, 32, 32, 0)
	return windows.Handle(hIcon)
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

	appendStr := func(h windows.Handle, flags uintptr, id uintptr, text string) {
		ptr, _ := windows.UTF16PtrFromString(text)
		pAppendMenu.Call(uintptr(h), flags, id, uintptr(unsafe.Pointer(ptr)))
	}

	headerText := "☁️ CloudflareNetConnect (🟢 Activo)"
	if !t.cfg.MasterToggle {
		headerText = "☁️ CloudflareNetConnect (🔴 Pausado)"
	}
	appendStr(hPopup, 0x0001, 0, headerText)
	appendStr(hPopup, 0x0800, 0, "") // Separator

	toggleFlag := uintptr(0x0000)
	if t.cfg.MasterToggle {
		toggleFlag = 0x0008 // MF_CHECKED
	}
	appendStr(hPopup, toggleFlag, idMaster, "Interruptor Maestro (ON / OFF)")
	appendStr(hPopup, 0x0800, 0, "") // Separator

	t.mu.Lock()
	toggleMap := make(map[uintptr]int)
	removeMap := make(map[uintptr]int)
	for i, acc := range t.cfg.Accounts {
		hSub, _, _ := pCreatePopupMenu.Call()
		hSubMenu := windows.Handle(hSub)

		accToggleID := uintptr(idAccToggleBase + i)
		accRemoveID := uintptr(idAccRemoveBase + i)
		toggleMap[accToggleID] = i
		removeMap[accRemoveID] = i

		flag := uintptr(0x0000)
		stateLabel := "[ ] Pausada (Habilitar)"
		if acc.Enabled {
			flag = 0x0008
			stateLabel = "[✓] Activa (Pausar)"
		}
		appendStr(hSubMenu, flag, accToggleID, stateLabel)
		appendStr(hSubMenu, 0x0800, 0, "")
		appendStr(hSubMenu, 0x0000, accRemoveID, "🗑️ Desvincular cuenta")

		iconPrefix := "🔴 "
		if acc.Enabled {
			iconPrefix = "🟢 "
		}
		appendStr(hPopup, 0x0010, uintptr(hSubMenu), iconPrefix+acc.Name) // MF_POPUP = 0x0010
	}
	t.mu.Unlock()

	appendStr(hPopup, 0x0800, 0, "")
	hAssocSub, _, _ := pCreatePopupMenu.Call()
	hAssocMenu := windows.Handle(hAssocSub)
	appendStr(hAssocMenu, 0x0000, idAssocOLR, "⚡ OLRSoft (SSO Automático 0-Click)")
	appendStr(hAssocMenu, 0x0000, idAssocHispa, "⚡ HispaServer (SSO Automático 0-Click)")
	appendStr(hAssocMenu, 0x0800, 0, "")
	appendStr(hAssocMenu, 0x0000, idAssociateArgo, "☁️ Otra cuenta Cloudflare (Elegir Dominio)...")
	appendStr(hPopup, 0x0010, uintptr(hAssocMenu), "➕ Asociar nueva cuenta...")
	appendStr(hPopup, 0x0800, 0, "")
	appendStr(hPopup, 0x0000, idQuit, "❌ Salir")

	var pt struct{ X, Y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(hWnd))

	cmd, _, _ := pTrackPopupMenu.Call(uintptr(hPopup), 0x0100|0x0002, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hWnd), 0)

	if idx, ok := toggleMap[cmd]; ok {
		t.mu.Lock()
		t.cfg.Accounts[idx].Enabled = !t.cfg.Accounts[idx].Enabled
		accID := t.cfg.Accounts[idx].ID
		state := t.cfg.Accounts[idx].Enabled
		_ = config.SaveConfig(t.configPath, t.cfg)
		t.mu.Unlock()
		t.orch.SetAccountEnabled(accID, state)
		log.Printf("[TRAY] Cuenta '%s' conmutada a: %v\n", accID, state)
		return
	}

	if idx, ok := removeMap[cmd]; ok {
		t.mu.Lock()
		accID := t.cfg.Accounts[idx].ID
		accName := t.cfg.Accounts[idx].Name
		t.cfg.Accounts = append(t.cfg.Accounts[:idx], t.cfg.Accounts[idx+1:]...)
		_ = config.SaveConfig(t.configPath, t.cfg)
		t.mu.Unlock()
		t.orch.RemoveAccount(accID)
		log.Printf("[TRAY] Cuenta '%s' (%s) desvinculada y eliminada con éxito.\n", accName, accID)
		return
	}

	switch cmd {
	case idMaster:
		t.cfg.MasterToggle = !t.cfg.MasterToggle
		t.orch.SetMasterToggle(t.cfg.MasterToggle)
		t.updateTrayIcon(t.cfg.MasterToggle)
		log.Printf("[TRAY] Master Toggle -> %v (icono actualizado)\n", t.cfg.MasterToggle)
	case idAssocOLR:
		t.StartAssociateFlow("olrsoft")
	case idAssocHispa:
		t.StartAssociateFlow("hispaserver")
	case idAssociateArgo:
		t.StartAssociateFlow("")
	case idQuit:
		log.Println("[TRAY] Cierre ordenado solicitado.")
		_ = t.orch.Stop()
		pPostQuitMessage.Call(0)
	}
}
