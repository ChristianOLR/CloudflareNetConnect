//go:build darwin

package tray

/*
#include <stdlib.h>

void stopMacLoop(void);
*/
import "C"

import (
	"os/exec"
	"path/filepath"

	"github.com/chgarcia79/cloudflarenetconnect/internal/cloudflare"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

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

//export goOnTogglePn
func goOnTogglePn(index int) {
	if g_darwinTray == nil {
		return
	}
	g_darwinTray.mu.Lock()
	if index >= 0 && index < len(g_darwinTray.cfg.Accounts) {
		acc := &g_darwinTray.cfg.Accounts[index]
		accID := acc.ID
		if acc.PrivateNetwork != nil {
			if acc.PrivateNetwork.TeamName == "" {
				client := cloudflare.NewClient(acc.APIToken)
				acc.PrivateNetwork.TeamName = client.DiscoverTeamName(acc.AccountID, acc.Name, acc.ZoneID)
			}
			if acc.PrivateNetwork.Token == "" || acc.PrivateNetwork.DeviceID == "" {
				acc.PrivateNetwork.DeviceID = ""
				_, _ = wireguard.EnsureEnrolled(acc.PrivateNetwork)
			}
		}
		g_darwinTray.mu.Unlock()

		if g_darwinTray.orch != nil {
			g_darwinTray.orch.TogglePrivateNetwork(accID)
			_ = config.SaveConfig(g_darwinTray.configPath, g_darwinTray.cfg)
		}
		g_darwinTray.refreshTrayState()
	} else {
		g_darwinTray.mu.Unlock()
	}
}

//export goOnTogglePnAuto
func goOnTogglePnAuto(index int) {
	if g_darwinTray == nil {
		return
	}
	g_darwinTray.mu.Lock()
	if index >= 0 && index < len(g_darwinTray.cfg.Accounts) {
		accID := g_darwinTray.cfg.Accounts[index].ID
		g_darwinTray.mu.Unlock()

		if g_darwinTray.orch != nil {
			g_darwinTray.orch.TogglePrivateNetworkAutoDetect(accID)
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
