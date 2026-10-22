package tunnel

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/olrsoft/cloudflarenetconnect/internal/config"
)

// Orchestrator gestiona el ciclo de vida concurrente de las conexiones por cuenta.
type Orchestrator struct {
	mu       sync.RWMutex
	cfg      *config.AppConfig
	cancels  map[string]context.CancelFunc
	active   bool
	globalCtx context.Context
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

func (o *Orchestrator) listenTCPRoute(ctx context.Context, accountID string, route config.TunnelRoute) {
	addr := fmt.Sprintf("127.0.0.1:%d", route.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[ROUTE-ERROR:%s] No se pudo escuchar en %s: %v\n", accountID, addr, err)
		return
	}
	defer listener.Close()

	log.Printf("[ROUTE-OK:%s] Enrutador local activo en %s -> %s\n", accountID, addr, route.Hostname)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		go o.handleTCPConnection(ctx, conn, route.Hostname)
	}
}

func (o *Orchestrator) handleTCPConnection(ctx context.Context, localConn net.Conn, targetHostname string) {
	defer localConn.Close()

	// Conexión puente hacia el host destino o cloudflared
	remoteConn, err := net.DialTimeout("tcp", targetHostname+":443", 5*time.Second)
	if err != nil {
		log.Printf("[TCP-ERR] Fallo de conexión hacia %s: %v\n", targetHostname, err)
		return
	}
	defer remoteConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}
}
