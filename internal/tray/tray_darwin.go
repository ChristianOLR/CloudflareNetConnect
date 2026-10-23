//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void initMacTray(const void* iconBytes, int iconLen);
void setMacMenu(int masterActive, const char* accountsJson, const void* iconBytes, int iconLen);
void runMacLoop(void);
void stopMacLoop(void);
*/
import "C"

import (
	"encoding/json"
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

	C.runMacLoop()
	log.Println("[TRAY] Bucle de eventos finalizado.")
}

func (t *TrayApp) refreshTrayState() {
	t.mu.Lock()
	defer t.mu.Unlock()

	type accSummary struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	summaries := make([]accSummary, len(t.cfg.Accounts))
	for i, a := range t.cfg.Accounts {
		summaries[i] = accSummary{Name: a.Name, Enabled: a.Enabled}
	}
	bytes, _ := json.Marshal(summaries)

	master := 0
	if t.cfg.MasterToggle {
		master = 1
	}

	png := t.currentIconBytes()
	cJson := C.CString(string(bytes))
	defer C.free(unsafe.Pointer(cJson))
	C.setMacMenu(C.int(master), cJson, unsafe.Pointer(&png[0]), C.int(len(png)))
}

//export goOnToggleMaster
func goOnToggleMaster() {
	if g_darwinTray == nil {
		return
	}
	g_darwinTray.mu.Lock()
	g_darwinTray.cfg.MasterToggle = !g_darwinTray.cfg.MasterToggle
	newState := g_darwinTray.cfg.MasterToggle
	g_darwinTray.mu.Unlock()

	_ = config.SaveConfig(g_darwinTray.configPath, g_darwinTray.cfg)
	if g_darwinTray.orch != nil {
		if newState {
			_ = g_darwinTray.orch.Start()
		} else {
			_ = g_darwinTray.orch.Stop()
		}
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
		accID := g_darwinTray.cfg.Accounts[index].AccountID
		enabled := g_darwinTray.cfg.Accounts[index].Enabled
		g_darwinTray.mu.Unlock()

		_ = config.SaveConfig(g_darwinTray.configPath, g_darwinTray.cfg)
		if g_darwinTray.orch != nil {
			g_darwinTray.orch.SetAccountEnabled(accID, enabled)
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
