//go:build windows

package tray

import (
	"fmt"
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
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
	maps := menuCommandMaps{
		toggleMap:   make(map[uintptr]int),
		removeMap:   make(map[uintptr]int),
		refreshMap:  make(map[uintptr]int),
		pnToggleMap: make(map[uintptr]int),
		pnAutoMap:   make(map[uintptr]int),
	}
	posIndex := uintptr(4)
	for i, acc := range t.cfg.Accounts {
		hSub, _, _ := pCreatePopupMenu.Call()
		hSubMenu := windows.Handle(hSub)

		accToggleID := uintptr(idAccToggleBase + i)
		accRemoveID := uintptr(idAccRemoveBase + i)
		accRefreshID := uintptr(idAccRefreshBase + i)
		accPnToggleID := uintptr(idAccPnToggleBase + i)
		accPnAutoID := uintptr(idAccPnAutoBase + i)

		maps.toggleMap[accToggleID] = i
		maps.removeMap[accRemoveID] = i
		maps.refreshMap[accRefreshID] = i
		maps.pnToggleMap[accPnToggleID] = i
		maps.pnAutoMap[accPnAutoID] = i

		flag := uintptr(0x0000)
		stateLabel := "Activar conexión de este dominio"
		if acc.Enabled {
			flag = 0x0008
			stateLabel = "Dominio Activo (Clic para Desactivar)"
		}
		appendStr(hSubMenu, flag, accToggleID, stateLabel)
		appendStr(hSubMenu, 0x0000, accRefreshID, "🔄 Actualizar túneles")

		// Red Privada WireGuard
		if t.orch != nil {
			pnStatus, pnCIDR, virtCIDR, autoDet, pnEnabled := t.orch.GetPrivateNetworkInfo(acc.ID)
			if pnCIDR != "" {
				pnFlag := uintptr(0x0000)
				pnLabel := fmt.Sprintf("🌐 Red Privada (%s)", pnCIDR)
				if pnEnabled {
					pnFlag = 0x0008
					if pnStatus == wireguard.StatusPausedLocal {
						pnLabel = fmt.Sprintf("🌐 Red Privada (%s) [🏠 Pausa LAN]", pnCIDR)
					} else if virtCIDR != "" {
						pnLabel = fmt.Sprintf("🌐 Red Privada (%s) [🔀 Virtual: %s]", pnCIDR, virtCIDR)
					} else {
						pnLabel = fmt.Sprintf("🌐 Red Privada (%s) [🟢 Activa]", pnCIDR)
					}
				}
				appendStr(hSubMenu, pnFlag, accPnToggleID, pnLabel)

				if pnEnabled {
					autoFlag := uintptr(0x0000)
					if autoDet {
						autoFlag = 0x0008
					}
					appendStr(hSubMenu, autoFlag, accPnAutoID, "   ↪ 🔄 Auto-Bypass en Red Local")
				}
			}
		}

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
	t.handleMenuCommand(cmd, maps)
}
