//go:build !windows

package tray

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/tunnel"
)

type TrayApp struct {
	mu         sync.Mutex
	orch       *tunnel.Orchestrator
	cfg        *config.AppConfig
	configPath string
}

func (t *TrayApp) refreshTrayState() {
	// No-op en entornos sin bandeja de sistema nativa Win32
}

func NewTrayApp(orch *tunnel.Orchestrator, cfg *config.AppConfig, configPath string) *TrayApp {
	return &TrayApp{orch: orch, cfg: cfg, configPath: configPath}
}

func (t *TrayApp) Run() {
	log.Println("[DAEMON] Modo demonio activo (Linux/macOS sin CGO).")
	if err := t.orch.Start(); err != nil {
		log.Fatalf("[DAEMON] Error al iniciar orquestador: %v", err)
	}

	if runtime.GOOS == "darwin" {
		go t.showMacControlDialog()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\n[DAEMON] Señal recibida. Deteniendo conexiones...")
	_ = t.orch.Stop()
	log.Println("[DAEMON] Finalizado con éxito.")
}

func (t *TrayApp) showMacControlDialog() {
	msg := fmt.Sprintf("CloudflareNetConnect está activo.\\nCuentas conectadas: %d", len(t.cfg.Accounts))
	if len(t.cfg.Accounts) == 0 {
		msg = "Bienvenido a CloudflareNetConnect.\\n\\nNo tienes ninguna cuenta de Cloudflare vinculada todavía."
	}

	script := fmt.Sprintf(`display dialog "%s" with title "CloudflareNetConnect" buttons {"Vincular Cuenta", "Abrir Config", "Aceptar"} default button "Aceptar"`, msg)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return
	}
	outStr := string(out)

	if strings.Contains(outStr, "Vincular Cuenta") {
		t.StartAssociateFlow()
	} else if strings.Contains(outStr, "Abrir Config") {
		_ = exec.Command("open", filepath.Dir(t.configPath)).Start()
	}
}
