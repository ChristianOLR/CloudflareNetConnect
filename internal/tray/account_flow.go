package tray

import (
	"log"
	"time"

	"github.com/chgarcia79/cloudflarenetconnect/internal/auth"
	"github.com/chgarcia79/cloudflarenetconnect/internal/cloudflare"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/ui"
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
		splash.SetSuccess(accName)

		t.mu.Lock()
		var targetAcc config.AccountConfig
		exists := false
		for i, a := range t.cfg.Accounts {
			if a.AccountID == accID || (a.Name == accName && accName != "") {
				t.cfg.Accounts[i].APIToken = res.Token
				t.cfg.Accounts[i].Name = accName
				t.cfg.Accounts[i].Enabled = true
				if len(discovered) > 0 {
					t.cfg.Accounts[i].Routes = mergeRoutes(t.cfg.Accounts[i].Routes, discovered)
				}
				targetAcc = t.cfg.Accounts[i]
				exists = true
				break
			}
		}

		if !exists {
			shortID := accID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			targetAcc = config.AccountConfig{
				ID:        shortID,
				Name:      accName,
				AccountID: accID,
				APIToken:  res.Token,
				Enabled:   true,
				Routes:    discovered,
			}
			t.cfg.Accounts = append(t.cfg.Accounts, targetAcc)
		}
		t.mu.Unlock()

		if err := config.SaveConfig(t.configPath, t.cfg); err != nil {
			log.Printf("[AUTH-FLOW] Error guardando configuración: %v\n", err)
		} else {
			log.Printf("[AUTH-FLOW] Configuración guardada (%d túneles descubiertos).\n", len(targetAcc.Routes))
		}

		if t.orch != nil {
			t.orch.ReloadAccount(targetAcc)
		}
		t.refreshTrayState()

		time.Sleep(1000 * time.Millisecond)
		splash.Close()
		cs.Close()
	}()
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
	discovered := client.DiscoverRoutes(acc.AccountID, "")

	t.mu.Lock()
	if index < len(t.cfg.Accounts) {
		t.cfg.Accounts[index].Routes = mergeRoutes(t.cfg.Accounts[index].Routes, discovered)
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

func mergeRoutes(existing, discovered []config.TunnelRoute) []config.TunnelRoute {
	if len(existing) == 0 {
		return ensureUniquePorts(discovered)
	}
	if len(discovered) == 0 {
		return ensureUniquePorts(existing)
	}
	byHost := make(map[string]config.TunnelRoute)
	for _, r := range existing {
		byHost[r.Hostname] = r
	}
	var merged []config.TunnelRoute
	seen := make(map[string]bool)
	for _, d := range discovered {
		if ex, ok := byHost[d.Hostname]; ok {
			d.Enabled = ex.Enabled
			if ex.LocalPort > 0 {
				d.LocalPort = ex.LocalPort
			}
		}
		merged = append(merged, d)
		seen[d.Hostname] = true
	}
	for _, r := range existing {
		if !seen[r.Hostname] {
			merged = append(merged, r)
		}
	}
	return ensureUniquePorts(merged)
}

func ensureUniquePorts(routes []config.TunnelRoute) []config.TunnelRoute {
	used := make(map[int]bool)
	for i := range routes {
		if routes[i].LocalPort <= 0 {
			continue
		}
		for used[routes[i].LocalPort] {
			routes[i].LocalPort++
		}
		used[routes[i].LocalPort] = true
	}
	return routes
}
