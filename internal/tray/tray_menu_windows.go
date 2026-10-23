//go:build windows

package tray

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"golang.org/x/sys/windows"
)

const idSvcStatusHeader = 1019

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
	refreshMap := make(map[uintptr]int)
	posIndex := uintptr(4)
	for i, acc := range t.cfg.Accounts {
		hSub, _, _ := pCreatePopupMenu.Call()
		hSubMenu := windows.Handle(hSub)

		accToggleID := uintptr(idAccToggleBase + i)
		accRemoveID := uintptr(idAccRemoveBase + i)
		accRefreshID := uintptr(idAccRefreshBase + i)
		toggleMap[accToggleID] = i
		removeMap[accRemoveID] = i
		refreshMap[accRefreshID] = i

		flag := uintptr(0x0000)
		stateLabel := "Activar conexión de este dominio"
		if acc.Enabled {
			flag = 0x0008
			stateLabel = "Dominio Activo (Clic para Desactivar)"
		}
		appendStr(hSubMenu, flag, accToggleID, stateLabel)
		appendStr(hSubMenu, 0x0000, accRefreshID, "🔄 Actualizar túneles")
		if len(acc.Routes) > 0 {
			appendStr(hSubMenu, 0x0800, 0, "")
			for _, r := range acc.Routes {
				appendStr(hSubMenu, 0x0001|0x0002, 0, fmt.Sprintf("🔌 %s → 127.0.0.1:%d", r.Hostname, r.LocalPort))
			}
		}
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
		posIndex++
	}
	appendStr(hPopup, 0x0000, idAssociate, "➕ Asociar nueva cuenta...")
	posIndex++

	// Submenú Servicio del Sistema (Windows / Autoarranque Dinámico)
	hSvc, _, _ := pCreatePopupMenu.Call()
	hSvcMenu := windows.Handle(hSvc)

	svcState := queryWindowsServiceStatus()
	isSvcInstalled := svcState != SvcNotInstalled

	// 1. Autoarranque en inicio de sesión (Bandeja con check nativo)
	autoFlag := uintptr(0x0000)
	autoText := "Iniciar aplicación al encender el PC"
	if isSvcInstalled {
		autoFlag = 0x0008 | 0x0001 | 0x0002 // MF_CHECKED | MF_GRAYED | MF_DISABLED
		autoText = "Iniciar al encender el PC (Gestionado por Servicio)"
	} else if isStartupEnabled() {
		autoFlag = 0x0008 // MF_CHECKED
	}
	appendStr(hSvcMenu, autoFlag, idStartupToggle, autoText)
	appendStr(hSvcMenu, 0x0800, 0, "") // Separador

	// 2. Estado del Servicio de Windows (Segundo plano real / Daemon)
	switch svcState {
	case SvcRunning:
		appendStr(hSvcMenu, 0x0000, idSvcStatusHeader, "Servicio Windows: Activo (2º Plano)")
		pSetMenuItemBitmaps.Call(uintptr(hSvcMenu), idSvcStatusHeader, 0, uintptr(t.hDotGreen), uintptr(t.hDotGreen))
		appendStr(hSvcMenu, 0x0800, 0, "")
		appendStr(hSvcMenu, 0x0000, idSvcStop, "⏹️ Detener Servicio Windows")
		appendStr(hSvcMenu, 0x0000, idSvcUninstall, "🗑️ Desinstalar Servicio Windows")
	case SvcStopped:
		appendStr(hSvcMenu, 0x0000, idSvcStatusHeader, "Servicio Windows: Detenido (Instalado)")
		pSetMenuItemBitmaps.Call(uintptr(hSvcMenu), idSvcStatusHeader, 0, uintptr(t.hDotRed), uintptr(t.hDotRed))
		appendStr(hSvcMenu, 0x0800, 0, "")
		appendStr(hSvcMenu, 0x0000, idSvcStart, "▶️ Iniciar Servicio Windows")
		appendStr(hSvcMenu, 0x0000, idSvcUninstall, "🗑️ Desinstalar Servicio Windows")
	default: // SvcNotInstalled
		appendStr(hSvcMenu, 0x0000, idSvcStatusHeader, "Servicio Windows: No Instalado")
		pSetMenuItemBitmaps.Call(uintptr(hSvcMenu), idSvcStatusHeader, 0, uintptr(t.hDotRed), uintptr(t.hDotRed))
		appendStr(hSvcMenu, 0x0800, 0, "")
		appendStr(hSvcMenu, 0x0000, idSvcInstall, "➕ Instalar y Activar Servicio Windows (2º Plano)")
	}

	svcMenuDot := t.hDotRed
	if svcState == SvcRunning || isStartupEnabled() {
		svcMenuDot = t.hDotGreen
	}
	appendStr(hPopup, 0x0010, uintptr(hSvcMenu), "⚙️ Servicio del Sistema (Autoarranque)")
	pSetMenuItemBitmaps.Call(uintptr(hPopup), posIndex, 0x0400, uintptr(svcMenuDot), uintptr(svcMenuDot))
	posIndex++

	appendStr(hPopup, 0x0800, 0, "")
	appendStr(hPopup, 0x0000, idQuit, "❌ Salir")

	var pt struct{ X, Y int32 }
	u32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))
	u32.NewProc("SetForegroundWindow").Call(uintptr(hWnd))

	cmd, _, _ := u32.NewProc("TrackPopupMenu").Call(uintptr(hPopup), 0x0100|0x0002, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hWnd), 0)
	t.handleMenuCommand(cmd, toggleMap, removeMap, refreshMap)
}

func (t *TrayApp) handleMenuCommand(cmd uintptr, toggleMap, removeMap, refreshMap map[uintptr]int) {
	if idx, ok := refreshMap[cmd]; ok {
		go t.RefreshAccountTunnels(idx)
		return
	}
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
		if queryWindowsServiceStatus() == SvcNotInstalled {
			setStartupEnabled(!isStartupEnabled(), exePath)
		}
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
