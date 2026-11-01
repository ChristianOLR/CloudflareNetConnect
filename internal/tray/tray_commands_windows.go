//go:build windows

package tray

import (
	"os"

	"github.com/chgarcia79/cloudflarenetconnect/internal/cloudflare"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
	"golang.org/x/sys/windows"
)

type menuCommandMaps struct {
	toggleMap   map[uintptr]int
	removeMap   map[uintptr]int
	refreshMap  map[uintptr]int
	pnToggleMap map[uintptr]int
	pnAutoMap   map[uintptr]int
}

func (t *TrayApp) handleMenuCommand(cmd uintptr, maps menuCommandMaps) {
	if idx, ok := maps.refreshMap[cmd]; ok {
		go t.RefreshAccountTunnels(idx)
		return
	}
	if idx, ok := maps.pnToggleMap[cmd]; ok {
		t.mu.Lock()
		acc := &t.cfg.Accounts[idx]
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
		t.mu.Unlock()
		if t.orch != nil {
			t.orch.TogglePrivateNetwork(accID)
			_ = config.SaveConfig(t.configPath, t.cfg)
		}
		t.refreshTrayState()
		return
	}
	if idx, ok := maps.pnAutoMap[cmd]; ok {
		t.mu.Lock()
		accID := t.cfg.Accounts[idx].ID
		t.mu.Unlock()
		if t.orch != nil {
			t.orch.TogglePrivateNetworkAutoDetect(accID)
			_ = config.SaveConfig(t.configPath, t.cfg)
		}
		t.refreshTrayState()
		return
	}
	if idx, ok := maps.toggleMap[cmd]; ok {
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
	if idx, ok := maps.removeMap[cmd]; ok {
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
