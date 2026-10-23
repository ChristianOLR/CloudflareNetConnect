package tunnel

import (
	"context"
	"log"
	"sync"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

// Orchestrator gestiona el ciclo de vida concurrente de las conexiones por cuenta.
type Orchestrator struct {
	mu           sync.RWMutex
	cfg          *config.AppConfig
	cancels      map[string]context.CancelFunc
	active       bool
	globalCtx    context.Context
	globalCancel context.CancelFunc
}

// NewOrchestrator instancia el orquestador multi-cuenta.
func NewOrchestrator(cfg *config.AppConfig) *Orchestrator {
	return &Orchestrator{
		cfg:     cfg,
		cancels: make(map[string]context.CancelFunc),
	}
}

// Start arranca todos los workers por cuenta respetando el MasterToggle.
func (o *Orchestrator) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.active {
		return nil
	}

	o.globalCtx, o.globalCancel = context.WithCancel(context.Background())
	o.active = true
	log.Println("[ORCHESTRATOR] Motor de túneles multi-cuenta iniciado.")

	if !o.cfg.MasterToggle {
		log.Println("[ORCHESTRATOR] Master Toggle en OFF. En reposo total (0% CPU).")
		return nil
	}

	for _, acc := range o.cfg.Accounts {
		if acc.Enabled {
			o.startAccountLocked(acc)
		}
	}
	return nil
}

// Stop apaga todos los workers y cierra sockets activos.
func (o *Orchestrator) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.active {
		return nil
	}

	log.Println("[ORCHESTRATOR] Deteniendo todas las conexiones concurrentes...")
	if o.globalCancel != nil {
		o.globalCancel()
	}

	for id, cancel := range o.cancels {
		cancel()
		delete(o.cancels, id)
	}

	o.active = false
	log.Println("[ORCHESTRATOR] Todos los sockets cerrados.")
	return nil
}

// SetMasterToggle conmuta el estado global de tráfico.
func (o *Orchestrator) SetMasterToggle(enabled bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.cfg.MasterToggle = enabled
	if !enabled {
		log.Println("[ORCHESTRATOR] Master Toggle -> OFF: Cerrando túneles activos.")
		for id, cancel := range o.cancels {
			cancel()
			delete(o.cancels, id)
		}
	} else if o.active {
		log.Println("[ORCHESTRATOR] Master Toggle -> ON: Reactivando túneles...")
		for _, acc := range o.cfg.Accounts {
			if acc.Enabled {
				o.startAccountLocked(acc)
			}
		}
	}
}

// SetAccountEnabled activa o desactiva una cuenta individual.
func (o *Orchestrator) SetAccountEnabled(accountID string, enabled bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !enabled {
		for _, acc := range o.cfg.Accounts {
			if acc.ID == accountID || acc.AccountID == accountID {
				if cancel, exists := o.cancels[acc.ID]; exists {
					log.Printf("[ORCHESTRATOR] Pausando cuenta: %s\n", acc.ID)
					cancel()
					delete(o.cancels, acc.ID)
				}
			}
		}
		if cancel, exists := o.cancels[accountID]; exists {
			cancel()
			delete(o.cancels, accountID)
		}
		return
	}

	if o.active && o.cfg.MasterToggle {
		for _, acc := range o.cfg.Accounts {
			if acc.ID == accountID || acc.AccountID == accountID {
				o.startAccountLocked(acc)
				break
			}
		}
	}
}

// ReloadAccount reinicia en caliente los túneles de una cuenta tras actualizar sus rutas.
func (o *Orchestrator) ReloadAccount(acc config.AccountConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if cancel, exists := o.cancels[acc.ID]; exists {
		cancel()
		delete(o.cancels, acc.ID)
	}
	if o.active && o.cfg.MasterToggle && acc.Enabled {
		o.startAccountLocked(acc)
	}
}

// RemoveAccount detiene los túneles de una cuenta eliminada.
func (o *Orchestrator) RemoveAccount(accountID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if cancel, exists := o.cancels[accountID]; exists {
		log.Printf("[ORCHESTRATOR] Desvinculando y cerrando cuenta: %s\n", accountID)
		cancel()
		delete(o.cancels, accountID)
	}
}

func (o *Orchestrator) startAccountLocked(acc config.AccountConfig) {
	if _, exists := o.cancels[acc.ID]; exists {
		return
	}

	ctx, cancel := context.WithCancel(o.globalCtx)
	o.cancels[acc.ID] = cancel

	go o.runAccountWorker(ctx, acc)
}

func (o *Orchestrator) runAccountWorker(ctx context.Context, acc config.AccountConfig) {
	log.Printf("[WORKER:%s] Iniciando pool para cuenta '%s'\n", acc.ID, acc.Name)

	for _, route := range acc.Routes {
		if route.Enabled {
			go o.listenTCPRoute(ctx, acc.ID, route)
		}
	}

	<-ctx.Done()
	log.Printf("[WORKER:%s] Pool finalizado limpiamente.\n", acc.ID)
}
