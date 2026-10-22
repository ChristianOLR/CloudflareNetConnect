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
	log.Println("[AUTH-FLOW] Iniciando flujo de vinculación de nueva cuenta...")

	cs, err := auth.NewCallbackServer()
	if err != nil {
		log.Printf("[AUTH-FLOW] Error iniciando servidor callback: %v\n", err)
		return
	}

	authURL := cs.GetAuthURL()
	splash := ui.NewSplashWindow(authURL)
	splash.Show()

	go func() {
		token, err := cs.WaitForToken(5 * time.Minute)
		splash.Close()

		if err != nil {
			log.Printf("[AUTH-FLOW] Error en autenticación: %v\n", err)
			return
		}

		if token == "" {
			log.Println("[AUTH-FLOW] Token vacío recibido.")
			return
		}

		log.Println("[AUTH-FLOW] Token recibido. Consultando organizaciones en Cloudflare API...")
		client := cloudflare.NewClient(token)
		accounts, err := client.ListAccounts()
		if err != nil || len(accounts) == 0 {
			log.Printf("[AUTH-FLOW] No se pudieron listar cuentas con el token: %v\n", err)
			return
		}

		primary := accounts[0]
		log.Printf("[AUTH-FLOW] ✅ Cuenta identificada: %s (ID: %s)\n", primary.Name, primary.ID)

		t.mu.Lock()
		// Verificar si ya existe
		exists := false
		for i, a := range t.cfg.Accounts {
			if a.AccountID == primary.ID {
				t.cfg.Accounts[i].APIToken = token
				t.cfg.Accounts[i].Enabled = true
				exists = true
				break
			}
		}

		if !exists {
			newAcc := config.AccountConfig{
				ID:        primary.ID[:8],
				Name:      primary.Name,
				AccountID: primary.ID,
				APIToken:  token,
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
	}()
}
