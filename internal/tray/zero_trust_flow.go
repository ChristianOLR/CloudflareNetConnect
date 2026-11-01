package tray

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"time"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/ui"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

// WatchForZeroTrustToken inicia la escucha reactiva del portapapeles y abre el navegador de autenticación.
func WatchForZeroTrustToken(teamName string, onToken func(token string)) {
	if teamName == "" {
		return
	}
	warpURL := fmt.Sprintf("https://%s.cloudflareaccess.com/warp", teamName)
	_ = ui.RegisterWarpProtocol()
	openWebBrowser(warpURL)
	log.Printf("[AUTH-ZT] 🌐 Abriendo navegador para validar Zero Trust: %s\n", warpURL)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		timeout := time.After(90 * time.Second)

		for {
			select {
			case <-timeout:
				return
			case <-ticker.C:
				clip := ui.ReadClipboardText()
				token := wireguard.ExtractZeroTrustToken(clip)
				if token != "" {
					log.Println("[AUTH-ZT] ✅ Token Zero Trust capturado automáticamente desde el portapapeles.")
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

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return false
	}

	updated := false
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
