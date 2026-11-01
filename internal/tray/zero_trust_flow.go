package tray

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/ui"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

var (
	activeZTSplash   *ui.SplashWindow
	activeZTSplashMu sync.Mutex
)

// WatchForZeroTrustToken inicia la ventana Splash y la escucha reactiva del portapapeles/protocolo.
func WatchForZeroTrustToken(teamName string, onToken func(token string)) {
	if teamName == "" {
		return
	}
	warpURL := fmt.Sprintf("https://%s.cloudflareaccess.com/warp", teamName)
	_ = ui.RegisterWarpProtocol()

	activeZTSplashMu.Lock()
	if activeZTSplash != nil {
		activeZTSplash.Close()
	}
	splash := ui.NewCustomSplashWindow(
		warpURL,
		"Autorizar Zero Trust (WARP)",
		"1. Abre o copia el enlace en tu navegador",
		"2. Introduce tu email y el código PIN",
	)
	activeZTSplash = splash
	activeZTSplashMu.Unlock()

	splash.Show()
	openWebBrowser(warpURL)
	log.Printf("[AUTH-ZT] 🌐 Mostrando Splash y abriendo navegador: %s\n", warpURL)

	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		timeout := time.After(3 * time.Minute)

		for {
			select {
			case <-timeout:
				log.Println("[AUTH-ZT] ⏱️ Tiempo agotado esperando autorización Zero Trust.")
				splash.Close()
				return
			case <-splash.Done():
				log.Println("[AUTH-ZT] Ventana Splash cerrada por el usuario.")
				return
			case <-ticker.C:
				clip := ui.ReadClipboardText()
				token := wireguard.ExtractZeroTrustToken(clip)
				if token != "" {
					log.Println("[AUTH-ZT] ✅ Token Zero Trust capturado automáticamente desde el portapapeles.")
					splash.SetSuccess("Zero Trust")
					time.Sleep(1200 * time.Millisecond)
					splash.Close()
					onToken(token)
					return
				}
			}
		}
	}()
}

// HandleIncomingWarpToken procesa una llamada com.cloudflare.warp:// recibida desde el sistema operativo.
func HandleIncomingWarpToken(rawToken, configPath string) bool {
	token := wireguard.ExtractZeroTrustToken(rawToken)
	if token == "" {
		return false
	}
	log.Println("[AUTH-ZT] 📥 Token Zero Trust recibido por argumento de sistema.")

	activeZTSplashMu.Lock()
	sp := activeZTSplash
	activeZTSplashMu.Unlock()
	if sp != nil {
		sp.SetSuccess("Zero Trust")
		go func() {
			time.Sleep(1200 * time.Millisecond)
			sp.Close()
		}()
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return false
	}

	updated := false
	for i := range cfg.Accounts {
		if cfg.Accounts[i].PrivateNetwork != nil && cfg.Accounts[i].PrivateNetwork.Enabled {
			cfg.Accounts[i].PrivateNetwork.ZeroTrustToken = token
			cfg.Accounts[i].PrivateNetwork.DeviceID = ""
			cfg.Accounts[i].PrivateNetwork.Token = ""
			_, _ = wireguard.EnsureEnrolled(cfg.Accounts[i].PrivateNetwork)
			updated = true
			break
		}
	}
	if !updated {
		for i := range cfg.Accounts {
			if cfg.Accounts[i].PrivateNetwork != nil {
				cfg.Accounts[i].PrivateNetwork.ZeroTrustToken = token
				cfg.Accounts[i].PrivateNetwork.DeviceID = ""
				cfg.Accounts[i].PrivateNetwork.Token = ""
				_, _ = wireguard.EnsureEnrolled(cfg.Accounts[i].PrivateNetwork)
				updated = true
				break
			}
		}
	}
	if updated {
		_ = config.SaveConfig(configPath, cfg)
		log.Println("[AUTH-ZT] 💾 Token Zero Trust guardado y dispositivo enrolado con éxito.")
		return true
	}
	return false
}

func openWebBrowser(targetURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	_ = cmd.Start()
}
