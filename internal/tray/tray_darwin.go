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
	"os/exec"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/assets"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/tunnel"
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
		Name    string   `json:"name"`
		Enabled bool     `json:"enabled"`
		Routes  []string `json:"routes"`
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
		summaries[i] = accSummary{Name: a.Name, Enabled: a.Enabled, Routes: rLabels}
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

//export goOnToggleMaster
func goOnToggleMaster() {
	if g_darwinTray == nil {
		return
	}
	g_darwinTray.mu.Lock()
	g_darwinTray.cfg.MasterToggle = !g_darwinTray.cfg.MasterToggle
	newState := g_darwinTray.cfg.MasterToggle
	_ = config.SaveConfig(g_darwinTray.configPath, g_darwinTray.cfg)
	g_darwinTray.mu.Unlock()

	if g_darwinTray.orch != nil {
		g_darwinTray.orch.SetMasterToggle(newState)
	}
	g_darwinTray.refreshTrayState()
}

//export goOnToggleAccount
func goOnToggleAccount(index int) {
	if g_darwinTray == nil {
		return
	}
	g_darwinTray.mu.Lock()
	if index >= 0 && index < len(g_darwinTray.cfg.Accounts) {
		g_darwinTray.cfg.Accounts[index].Enabled = !g_darwinTray.cfg.Accounts[index].Enabled
		accID := g_darwinTray.cfg.Accounts[index].ID
		enabled := g_darwinTray.cfg.Accounts[index].Enabled
		_ = config.SaveConfig(g_darwinTray.configPath, g_darwinTray.cfg)
		g_darwinTray.mu.Unlock()

		if g_darwinTray.orch != nil {
			g_darwinTray.orch.SetAccountEnabled(accID, enabled)
		}
		g_darwinTray.refreshTrayState()
	} else {
		g_darwinTray.mu.Unlock()
	}
}

//export goOnRefreshAccount
func goOnRefreshAccount(index int) {
	if g_darwinTray != nil {
		go g_darwinTray.RefreshAccountTunnels(index)
	}
}

//export goOnRemoveAccount
func goOnRemoveAccount(index int) {
	if g_darwinTray == nil {
		return
	}
	g_darwinTray.mu.Lock()
	if index >= 0 && index < len(g_darwinTray.cfg.Accounts) {
		accID := g_darwinTray.cfg.Accounts[index].ID
		g_darwinTray.cfg.Accounts = append(g_darwinTray.cfg.Accounts[:index], g_darwinTray.cfg.Accounts[index+1:]...)
		_ = config.SaveConfig(g_darwinTray.configPath, g_darwinTray.cfg)
		g_darwinTray.mu.Unlock()

		if g_darwinTray.orch != nil {
			g_darwinTray.orch.RemoveAccount(accID)
		}
		g_darwinTray.refreshTrayState()
	} else {
		g_darwinTray.mu.Unlock()
	}
}

//export goOnLinkAccount
func goOnLinkAccount() {
	if g_darwinTray != nil {
		go g_darwinTray.StartAssociateFlow()
	}
}

//export goOnToggleStartup
func goOnToggleStartup() {
	if g_darwinTray != nil {
		if queryMacServiceStatus() == macSvcNotInstalled {
			setMacStartupEnabled(!isMacStartupEnabled())
			g_darwinTray.refreshTrayState()
		}
	}
}

//export goOnServiceAction
func goOnServiceAction(cAction *C.char) {
	if g_darwinTray != nil && cAction != nil {
		act := C.GoString(cAction)
		go func() {
			runMacElevatedServiceCmd(act)
			g_darwinTray.refreshTrayState()
		}()
	}
}

//export goOnOpenConfig
func goOnOpenConfig() {
	if g_darwinTray != nil {
		_ = exec.Command("open", filepath.Dir(g_darwinTray.configPath)).Start()
	}
}

//export goOnQuit
func goOnQuit() {
	if g_darwinTray != nil && g_darwinTray.orch != nil {
		_ = g_darwinTray.orch.Stop()
	}
	C.stopMacLoop()
}
