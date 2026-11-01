package tray

import (
	"fmt"
	"log"
	"time"

	"github.com/chgarcia79/cloudflarenetconnect/internal/auth"
	"github.com/chgarcia79/cloudflarenetconnect/internal/cloudflare"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/ui"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

func (t *TrayApp) StartAssociateFlow() {
	log.Println("[AUTH-FLOW] Iniciando flujo universal de vinculación de cuenta...")

	cs, err := auth.NewCallbackServer()
	if err != nil {
		log.Printf("[AUTH-FLOW] Error iniciando transferencia: %v\n", err)
		return
	}

	splash := ui.NewSplashWindow(cs.GetAuthURL())
	splash.Show()

	go func() {
		splash.Wait()
		cs.Close()
	}()

	go func() {
		res, err := cs.WaitForToken(8 * time.Minute)
		if err != nil || res.Token == "" {
			log.Printf("[AUTH-FLOW] Flujo finalizado sin token: %v\n", err)
			splash.Close()
			cs.Close()
			return
		}

		client := cloudflare.NewClient(res.Token)
		accID, accName := client.ResolveAccountInfo(res.AccountID, res.ZoneID)
		log.Printf("[AUTH-FLOW] ✅ Cuenta identificada: %s (ID: %s)\n", accName, accID)

		discovered := client.DiscoverRoutes(accID, res.ZoneID)
		pnCIDR, hasPN := client.DiscoverPrivateCIDR(accID, res.ZoneID)
		teamName := client.DiscoverTeamName(accID, accName, res.ZoneID)
		splash.SetSuccess(accName)

		t.mu.Lock()
		targetAcc := t.applyDiscoveredAccount(res, accID, accName, discovered, pnCIDR, teamName, hasPN)
		t.mu.Unlock()

		_ = config.SaveConfig(t.configPath, t.cfg)
		if t.orch != nil {
			t.orch.ReloadAccount(targetAcc)
		}
		t.refreshTrayState()

		time.Sleep(1000 * time.Millisecond)
		splash.Close()
		cs.Close()
	}()
}

func (t *TrayApp) applyDiscoveredAccount(res auth.AuthResult, accID, accName string, discovered []config.TunnelRoute, pnCIDR, teamName string, hasPN bool) config.AccountConfig {
	for i, a := range t.cfg.Accounts {
		isMatch := (res.ZoneID != "" && a.ZoneID == res.ZoneID) || (a.Name == accName && accName != "") || (a.AccountID == accID)
		if isMatch {
			t.cfg.Accounts[i].APIToken = res.Token
			t.cfg.Accounts[i].Name = accName
			t.cfg.Accounts[i].ZoneID = res.ZoneID
			t.cfg.Accounts[i].Enabled = true
			if len(discovered) > 0 {
				t.cfg.Accounts[i].Routes = mergeRoutes(t.cfg.Accounts[i].Routes, discovered)
			}
			if hasPN {
				t.ensurePrivateNetworkConfig(&t.cfg.Accounts[i], pnCIDR, teamName)
			}
			return t.cfg.Accounts[i]
		}
	}

	shortID := res.ZoneID
	if shortID == "" {
		shortID = accID
	}
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	for _, a := range t.cfg.Accounts {
		if a.ID == shortID {
			shortID = fmt.Sprintf("%s-%d", shortID, len(t.cfg.Accounts))
			break
		}
	}
	targetAcc := config.AccountConfig{
		ID:        shortID,
		Name:      accName,
		AccountID: accID,
		ZoneID:    res.ZoneID,
		APIToken:  res.Token,
		Enabled:   true,
		Routes:    discovered,
	}
	if hasPN {
		t.ensurePrivateNetworkConfig(&targetAcc, pnCIDR, teamName)
	}
	t.cfg.Accounts = append(t.cfg.Accounts, targetAcc)
	return targetAcc
}

func (t *TrayApp) ensurePrivateNetworkConfig(acc *config.AccountConfig, pnCIDR, teamName string) {
	if acc.PrivateNetwork == nil {
		acc.PrivateNetwork = &config.PrivateNetworkConfig{
			Enabled:    false,
			AutoDetect: true,
			CIDR:       pnCIDR,
			TeamName:   teamName,
		}
	} else {
		acc.PrivateNetwork.CIDR = pnCIDR
		if acc.PrivateNetwork.TeamName == "" {
			acc.PrivateNetwork.TeamName = teamName
		}
	}
	if acc.PrivateNetwork.DeviceID == "" || acc.PrivateNetwork.Token == "" {
		acc.PrivateNetwork.DeviceID = ""
		if enrolled, err := wireguard.EnsureEnrolled(acc.PrivateNetwork); err == nil && enrolled != nil {
			log.Printf("[AUTH-FLOW] 🛡️ Red privada aprovisionada en Zero Trust (Org: %s, IP: %s)\n", teamName, enrolled.AddressIPv4)
		}
	}
}

// RefreshAccountTunnels consulta a la API de Cloudflare los túneles de una cuenta y recarga sus workers.
func (t *TrayApp) RefreshAccountTunnels(index int) {
	t.mu.Lock()
	if index < 0 || index >= len(t.cfg.Accounts) {
		t.mu.Unlock()
		return
	}
	acc := t.cfg.Accounts[index]
	t.mu.Unlock()

	if acc.APIToken == "" {
		log.Printf("[TUNNEL-SYNC] La cuenta '%s' no tiene token API.\n", acc.Name)
		return
	}

	log.Printf("[TUNNEL-SYNC] 🔄 Actualizando túneles desde Cloudflare para '%s'...\n", acc.Name)
	client := cloudflare.NewClient(acc.APIToken)
	discovered := client.DiscoverRoutes(acc.AccountID, acc.ZoneID)
	pnCIDR, hasPN := client.DiscoverPrivateCIDR(acc.AccountID, acc.ZoneID)
	teamName := client.DiscoverTeamName(acc.AccountID, acc.Name, acc.ZoneID)

	t.mu.Lock()
	if index < len(t.cfg.Accounts) {
		t.cfg.Accounts[index].Routes = mergeRoutes(t.cfg.Accounts[index].Routes, discovered)
		if hasPN {
			t.ensurePrivateNetworkConfig(&t.cfg.Accounts[index], pnCIDR, teamName)
			log.Printf("[TUNNEL-SYNC] 🌐 Red privada activa para '%s': %s (Org: %s)\n", acc.Name, pnCIDR, teamName)
		} else if t.cfg.Accounts[index].PrivateNetwork != nil && !t.cfg.Accounts[index].PrivateNetwork.Enabled && (t.cfg.Accounts[index].PrivateNetwork.CIDR == "192.168.1.0/24" || t.cfg.Accounts[index].PrivateNetwork.CIDR == "") {
			t.cfg.Accounts[index].PrivateNetwork = nil
		}
		acc = t.cfg.Accounts[index]
		_ = config.SaveConfig(t.configPath, t.cfg)
	}
	t.mu.Unlock()

	log.Printf("[TUNNEL-SYNC] ✅ '%s' sincronizada con %d rutas activas.\n", acc.Name, len(acc.Routes))
	if t.orch != nil {
		t.orch.ReloadAccount(acc)
	}
	t.refreshTrayState()
}

// AutoSyncMissingRoutes sincroniza al inicio cualquier cuenta que tenga APIToken configurado.
func (t *TrayApp) AutoSyncMissingRoutes() {
	t.mu.Lock()
	count := len(t.cfg.Accounts)
	t.mu.Unlock()
	for i := 0; i < count; i++ {
		t.mu.Lock()
		needsSync := i < len(t.cfg.Accounts) && t.cfg.Accounts[i].APIToken != ""
		t.mu.Unlock()
		if needsSync {
			t.RefreshAccountTunnels(i)
		}
	}
}
