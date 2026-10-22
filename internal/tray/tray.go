package tray

import (
	"log"

	"github.com/olrsoft/cloudflarenetconnect/internal/tunnel"
)

// TrayHandler abstrae la interacción con la bandeja del sistema.
type TrayHandler struct {
	orch *tunnel.Orchestrator
}

// NewTrayHandler crea una nueva instancia del gestor de bandeja.
func NewTrayHandler(orch *tunnel.Orchestrator) *TrayHandler {
	return &TrayHandler{orch: orch}
}

// ToggleMaster cambia el estado del interruptor maestro.
func (t *TrayHandler) ToggleMaster(enable bool) {
	log.Printf("[TRAY] Conmutador Maestro accionado: %v\n", enable)
	t.orch.SetMasterToggle(enable)
}

// OnQuit realiza la limpieza previa al cierre.
func (t *TrayHandler) OnQuit() {
	log.Println("[TRAY] Solicitado cierre de CloudflareNetConnect.")
	_ = t.orch.Stop()
}
