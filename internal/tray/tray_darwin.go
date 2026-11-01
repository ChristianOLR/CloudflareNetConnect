//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void initMacTray(const void* iconBytes, int iconLen);
void setMacMenu(const char* stateJson, const void* iconBytes, int iconLen);
void runMacLoop(void);
void stopMacLoop(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/assets"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/tunnel"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

var g_darwinTray *TrayApp

type TrayApp struct {
	mu         sync.Mutex
	orch       *tunnel.Orchestrator
	cfg        *config.AppConfig
	configPath string
}

func NewTrayApp(orch *tunnel.Orchestrator, cfg *config.AppConfig, configPath string) *TrayApp {
	app := &TrayApp{orch: orch, cfg: cfg, configPath: configPath}
	g_darwinTray = app
	return app
}

func (t *TrayApp) isEffectivelyActive() bool {
	if !t.cfg.MasterToggle {
		return false
	}
	for _, acc := range t.cfg.Accounts {
		if acc.Enabled {
			return true
		}
	}
	return false
}

func (t *TrayApp) currentIconBytes() []byte {
	if t.isEffectivelyActive() {
		return assets.CloudflareGreenPNG
	}
	return assets.CloudflareRedPNG
}

func (t *TrayApp) Run() {
	log.Println("[TRAY] Iniciando System Tray nativo de macOS (NSStatusItem)...")
	png := t.currentIconBytes()
	C.initMacTray(unsafe.Pointer(&png[0]), C.int(len(png)))

	t.refreshTrayState()
	go t.AutoSyncMissingRoutes()

	C.runMacLoop()
	log.Println("[TRAY] Bucle de eventos finalizado.")
}

func (t *TrayApp) refreshTrayState() {
	t.mu.Lock()
	type accSummary struct {
		Name         string   `json:"name"`
		Enabled      bool     `json:"enabled"`
		Routes       []string `json:"routes"`
		PnEnabled    bool     `json:"pn_enabled"`
		PnLabel      string   `json:"pn_label"`
		PnAutoDetect bool     `json:"pn_auto_detect"`
		PnShowAuto   bool     `json:"pn_show_auto"`
	}
	type menuStatePayload struct {
		MasterActive   bool         `json:"master_active"`
		StartupEnabled bool         `json:"startup_enabled"`
		SvcState       int          `json:"svc_state"`
		Accounts       []accSummary `json:"accounts"`
	}

	summaries := make([]accSummary, len(t.cfg.Accounts))
	for i, a := range t.cfg.Accounts {
		rLabels := make([]string, 0, len(a.Routes))
		for _, r := range a.Routes {
			rLabels = append(rLabels, fmt.Sprintf("🔌 %s → 127.0.0.1:%d", r.Hostname, r.LocalPort))
		}

		pnLabel := "🌐 Red Privada"
		pnEnabled := false
		pnAutoDetect := false
		pnShowAuto := false

		if t.orch != nil {
			status, cidr, autoDet, en := t.orch.GetPrivateNetworkInfo(a.ID)
			if cidr == "" {
				cidr = "192.168.1.0/24"
			}
			pnEnabled = en
			pnAutoDetect = autoDet
			pnShowAuto = en
			if en {
				if status == wireguard.StatusPausedLocal {
					pnLabel = fmt.Sprintf("🌐 Red Privada (%s) [🏠 Pausa LAN]", cidr)
				} else {
					pnLabel = fmt.Sprintf("🌐 Red Privada (%s) [🟢 Activa]", cidr)
				}
			} else {
				pnLabel = fmt.Sprintf("🌐 Red Privada (%s)", cidr)
			}
		}

		summaries[i] = accSummary{
			Name:         a.Name,
			Enabled:      a.Enabled,
			Routes:       rLabels,
			PnEnabled:    pnEnabled,
			PnLabel:      pnLabel,
			PnAutoDetect: pnAutoDetect,
			PnShowAuto:   pnShowAuto,
		}
	}
	payload := menuStatePayload{
		MasterActive:   t.cfg.MasterToggle,
		StartupEnabled: isMacStartupEnabled(),
		SvcState:       queryMacServiceStatus(),
		Accounts:       summaries,
	}
	png := t.currentIconBytes()
	t.mu.Unlock()

	bytes, _ := json.Marshal(payload)
	cJson := C.CString(string(bytes))
	defer C.free(unsafe.Pointer(cJson))
	C.setMacMenu(cJson, unsafe.Pointer(&png[0]), C.int(len(png)))
}
