//go:build windows

package tray

import (
	"log"
	"os"
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
	idHeader        = 1000
	idMaster        = 1001
	idAssociate     = 1002
	idQuit          = 1003
	idStartupToggle = 1010
	idSvcInstall    = 1011
	idSvcStart      = 1012
	idSvcStop       = 1013
	idSvcUninstall  = 1014
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
	hWin, _, _ := u32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(clsName)), uintptr(unsafe.Pointer(wName)), 0x00CF0000, 0, 0, 0, 0, 0, 0, hInst, 0)
	t.hWnd = windows.Handle(hWin)

	t.nid = NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(t.nid)),
		HWnd:             t.hWnd,
		UID:              100,
		UFlags:           7,
		UCallbackMessage: wmAppTray,
		HIcon:            t.hIcon,
	}

	s32.NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&t.nid)))
	t.refreshTrayState()
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
		ret, _, _ := u32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		u32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		u32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}

	s32.NewProc("Shell_NotifyIconW").Call(2, uintptr(unsafe.Pointer(&t.nid)))
	u32.NewProc("DestroyWindow").Call(uintptr(t.hWnd))
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

func (t *TrayApp) showMenu(hWnd windows.Handle) {
	u32 := windows.NewLazySystemDLL("user32.dll")
	pCreatePopupMenu := u32.NewProc("CreatePopupMenu")
	pAppendMenu := u32.NewProc("AppendMenuW")
	pSetMenuItemBitmaps := u32.NewProc("SetMenuItemBitmaps")

	hMenu, _, _ := pCreatePopupMenu.Call()
	hPopup := windows.Handle(hMenu)

	appendStr := func(h windows.Handle, flags uintptr, id uintptr, text string) {
		ptr, _ := windows.UTF16PtrFromString(text)
		pAppendMenu.Call(uintptr(h), flags, id, uintptr(unsafe.Pointer(ptr)))
	}

	t.mu.Lock()
	effectivelyActive := t.isEffectivelyActive()
	hasAccounts := len(t.cfg.Accounts) > 0
	t.mu.Unlock()

	headerText := "CloudflareNetConnect (Sin dominios activos)"
	headerDot := t.hDotRed
	if effectivelyActive {
		headerText = "CloudflareNetConnect (Servicio Activo)"
		headerDot = t.hDotGreen
	} else if !t.cfg.MasterToggle {
		headerText = "CloudflareNetConnect (Servicio Desactivado)"
	}
	appendStr(hPopup, 0x0000, idHeader, headerText)
	pSetMenuItemBitmaps.Call(uintptr(hPopup), idHeader, 0, uintptr(headerDot), uintptr(headerDot))
	appendStr(hPopup, 0x0800, 0, "")

	masterLabel := "⚡ Activar Servicio de Túneles (Actualmente OFF)"
	masterFlag := uintptr(0x0000)
	if t.cfg.MasterToggle {
		masterLabel = "⚡ Servicio de Túneles Activo (Clic para Desactivar)"
		masterFlag = 0x0008
	}
	appendStr(hPopup, masterFlag, idMaster, masterLabel)
	appendStr(hPopup, 0x0800, 0, "")

	t.mu.Lock()
	toggleMap := make(map[uintptr]int)
	removeMap := make(map[uintptr]int)
	posIndex := uintptr(4)
	for i, acc := range t.cfg.Accounts {
		hSub, _, _ := pCreatePopupMenu.Call()
		hSubMenu := windows.Handle(hSub)

		accToggleID := uintptr(idAccToggleBase + i)
		accRemoveID := uintptr(idAccRemoveBase + i)
		toggleMap[accToggleID] = i
		removeMap[accRemoveID] = i

		flag := uintptr(0x0000)
		stateLabel := "Activar conexión de este dominio"
		if acc.Enabled {
			flag = 0x0008
			stateLabel = "Dominio Activo (Clic para Desactivar)"
		}
		appendStr(hSubMenu, flag, accToggleID, stateLabel)
		appendStr(hSubMenu, 0x0800, 0, "")
		appendStr(hSubMenu, 0x0000, accRemoveID, "🗑️ Desvincular cuenta")

		appendStr(hPopup, 0x0010, uintptr(hSubMenu), acc.Name)
		dot := t.hDotRed
		if acc.Enabled && t.cfg.MasterToggle {
			dot = t.hDotGreen
		}
		pSetMenuItemBitmaps.Call(uintptr(hPopup), posIndex, 0x0400, uintptr(dot), uintptr(dot))
		posIndex++
	}
	t.mu.Unlock()

	if hasAccounts {
		appendStr(hPopup, 0x0800, 0, "")
	}
	appendStr(hPopup, 0x0000, idAssociate, "➕ Asociar nueva cuenta...")

	// Submenú Servicio del Sistema (Windows / Autoarranque)
	hSvc, _, _ := pCreatePopupMenu.Call()
	hSvcMenu := windows.Handle(hSvc)
	autoFlag := uintptr(0x0000)
	if isStartupEnabled() {
		autoFlag = 0x0008
	}
	appendStr(hSvcMenu, autoFlag, idStartupToggle, "Iniciar automáticamente con el sistema")
	appendStr(hSvcMenu, 0x0800, 0, "")
	appendStr(hSvcMenu, 0x0000, idSvcInstall, "Instalar y Activar Servicio Windows (2º Plano)")
	appendStr(hSvcMenu, 0x0000, idSvcStart, "Iniciar Servicio Windows")
	appendStr(hSvcMenu, 0x0000, idSvcStop, "Detener Servicio Windows")
	appendStr(hSvcMenu, 0x0000, idSvcUninstall, "Desinstalar Servicio Windows")
	appendStr(hPopup, 0x0010, uintptr(hSvcMenu), "⚙️ Servicio del Sistema (Autoarranque)")

	appendStr(hPopup, 0x0800, 0, "")
	appendStr(hPopup, 0x0000, idQuit, "❌ Salir")

	var pt struct{ X, Y int32 }
	u32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))
	u32.NewProc("SetForegroundWindow").Call(uintptr(hWnd))

	cmd, _, _ := u32.NewProc("TrackPopupMenu").Call(uintptr(hPopup), 0x0100|0x0002, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hWnd), 0)
	t.handleMenuCommand(cmd, toggleMap, removeMap)
}

func (t *TrayApp) handleMenuCommand(cmd uintptr, toggleMap, removeMap map[uintptr]int) {
	if idx, ok := toggleMap[cmd]; ok {
		t.mu.Lock()
		t.cfg.Accounts[idx].Enabled = !t.cfg.Accounts[idx].Enabled
		accID := t.cfg.Accounts[idx].ID
		state := t.cfg.Accounts[idx].Enabled
		_ = config.SaveConfig(t.configPath, t.cfg)
		t.mu.Unlock()
		t.orch.SetAccountEnabled(accID, state)
		t.refreshTrayState()
		return
	}
	if idx, ok := removeMap[cmd]; ok {
		t.mu.Lock()
		accID := t.cfg.Accounts[idx].ID
		t.cfg.Accounts = append(t.cfg.Accounts[:idx], t.cfg.Accounts[idx+1:]...)
		_ = config.SaveConfig(t.configPath, t.cfg)
		t.mu.Unlock()
		t.orch.RemoveAccount(accID)
		t.refreshTrayState()
		return
	}
	exePath, _ := os.Executable()
	switch cmd {
	case idMaster:
		t.cfg.MasterToggle = !t.cfg.MasterToggle
		_ = config.SaveConfig(t.configPath, t.cfg)
		t.orch.SetMasterToggle(t.cfg.MasterToggle)
		t.refreshTrayState()
	case idAssociate:
		t.StartAssociateFlow()
	case idStartupToggle:
		setStartupEnabled(!isStartupEnabled(), exePath)
	case idSvcInstall:
		runElevatedServiceCmd(exePath, "install")
		runElevatedServiceCmd(exePath, "start")
	case idSvcStart:
		runElevatedServiceCmd(exePath, "start")
	case idSvcStop:
		runElevatedServiceCmd(exePath, "stop")
	case idSvcUninstall:
		runElevatedServiceCmd(exePath, "stop")
		runElevatedServiceCmd(exePath, "uninstall")
	case idQuit:
		_ = t.orch.Stop()
		windows.NewLazySystemDLL("user32.dll").NewProc("PostQuitMessage").Call(0)
	}
}
