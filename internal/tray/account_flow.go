//go:build windows

package tray

import (
	"fmt"
	"log"
	"time"

	"github.com/olrsoft/cloudflarenetconnect/internal/auth"
	"github.com/olrsoft/cloudflarenetconnect/internal/cloudflare"
	"github.com/olrsoft/cloudflarenetconnect/internal/config"
	"github.com/olrsoft/cloudflarenetconnect/internal/ui"
)

func (t *TrayApp) StartAssociateFlow(presetKey string) {
	log.Printf("[AUTH-FLOW] Iniciando flujo de vinculación (preset='%s')...\n", presetKey)

	cs, err := auth.NewCallbackServer(presetKey)
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

		accID := res.AccountID
		accName := res.AccountName
		if accName != "" && res.UserEmail != "" {
			accName = fmt.Sprintf("%s [%s]", res.AccountName, res.UserEmail)
		}
		if accName == "" {
			client := cloudflare.NewClient(res.Token)
			accID, accName = client.ResolveAccountInfo(res.AccountID, res.ZoneID)
		}
		log.Printf("[AUTH-FLOW] ✅ Cuenta identificada: %s (ID: %s)\n", accName, accID)

		splash.SetSuccess(accName)

		t.mu.Lock()
		exists := false
		for i, a := range t.cfg.Accounts {
			if a.AccountID == accID || (a.Name == accName && accName != "") {
				t.cfg.Accounts[i].APIToken = res.Token
				t.cfg.Accounts[i].Name = accName
				t.cfg.Accounts[i].Enabled = true
				exists = true
				break
			}
		}

		if !exists {
			shortID := accID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			var defaultRoutes []config.TunnelRoute
			if presetKey == "olrsoft" || accID == "9940795d76034a3a83ec23ab25311965" {
				defaultRoutes = []config.TunnelRoute{
					{
						Hostname:  "syncnetdb.olrcloud.com",
						LocalPort: 5432,
						Protocol:  "tcp",
						Enabled:   true,
					},
				}
			}
			newAcc := config.AccountConfig{
				ID:        shortID,
				Name:      accName,
				AccountID: accID,
				APIToken:  res.Token,
				Enabled:   true,
				Routes:    defaultRoutes,
			}
			t.cfg.Accounts = append(t.cfg.Accounts, newAcc)
		}
		t.mu.Unlock()

		if err := config.SaveConfig(t.configPath, t.cfg); err != nil {
			log.Printf("[AUTH-FLOW] Error guardando configuración: %v\n", err)
		} else {
			log.Println("[AUTH-FLOW] Configuración actualizada en disco con la nueva cuenta.")
		}

		if t.orch != nil {
			t.orch.SetAccountEnabled(accID, true)
		}
		t.updateTrayIcon(t.cfg.MasterToggle)

		time.Sleep(1000 * time.Millisecond)
		splash.Close()
		cs.Close()
	}()
}
