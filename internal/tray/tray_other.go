//go:build !windows

package tray

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/olrsoft/cloudflarenetconnect/internal/config"
	"github.com/olrsoft/cloudflarenetconnect/internal/tunnel"
)

// TrayApp maneja el modo demonio headless en Linux y macOS cuando no hay entorno CGO/GUI.
type TrayApp struct {
	orch *tunnel.Orchestrator
	cfg  *config.AppConfig
}

// NewTrayApp crea la instancia en modo demonio.
func NewTrayApp(orch *tunnel.Orchestrator, cfg *config.AppConfig) *TrayApp {
	return &TrayApp{orch: orch, cfg: cfg}
}

// Run inicia el bucle de ejecución basado en señales del sistema.
func (t *TrayApp) Run() {
	log.Println("[DAEMON] Modo demonio activo (Linux/macOS sin CGO).")
	if err := t.orch.Start(); err != nil {
		log.Fatalf("[DAEMON] Error al iniciar orquestador: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\n[DAEMON] Señal recibida. Deteniendo conexiones...")
	_ = t.orch.Stop()
	log.Println("[DAEMON] Finalizado con éxito.")
}
