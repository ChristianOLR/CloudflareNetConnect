//go:build windows

package tray

import (
	"log"
	"time"

	"github.com/olrsoft/cloudflarenetconnect/internal/auth"
	"github.com/olrsoft/cloudflarenetconnect/internal/cloudflare"
	"github.com/olrsoft/cloudflarenetconnect/internal/config"
	"github.com/olrsoft/cloudflarenetconnect/internal/ui"
)

func (t *TrayApp) StartAssociateFlow() {
	log.Println("[AUTH-FLOW] Iniciando flujo de vinculación de nueva cuenta (Cloudflare Argo Transfer)...")

	cs, err := auth.NewCallbackServer()
	if err != nil {
		log.Printf("[AUTH-FLOW] Error iniciando transferencia Argo: %v\n", err)
		return
	}

	splash := ui.NewSplashWindow(cs.GetAuthURL())
	splash.Show()

	// Si el usuario cierra o cancela la tarjeta Splash, abortamos el polling
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

		log.Println("[AUTH-FLOW] Credencial recibida. Resolviendo detalles de organización en Cloudflare API...")
		client := cloudflare.NewClient(res.Token)
		accID, accName := client.ResolveAccountInfo(res.AccountID, res.ZoneID)
		log.Printf("[AUTH-FLOW] ✅ Cuenta identificada: %s (ID: %s)\n", accName, accID)

		splash.SetSuccess(accName)

		t.mu.Lock()
		exists := false
		for i, a := range t.cfg.Accounts {
			if a.AccountID == accID || (a.Name == accName && accName != "") {
				t.cfg.Accounts[i].APIToken = res.Token
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
			newAcc := config.AccountConfig{
				ID:        shortID,
				Name:      accName,
				AccountID: accID,
				APIToken:  res.Token,
				Enabled:   true,
				Routes:    []config.TunnelRoute{},
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
