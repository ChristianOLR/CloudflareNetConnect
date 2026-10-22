package service

import (
	"fmt"
	"log"

	"github.com/kardianos/service"
)

// Runner define la interfaz que ejecuta la lógica del daemon.
type Runner interface {
	Start() error
	Stop() error
}

type program struct {
	runner Runner
}

func (p *program) Start(s service.Service) error {
	go func() {
		if err := p.runner.Start(); err != nil {
			log.Printf("[SERVICE] Error al arrancar: %v", err)
		}
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	return p.runner.Stop()
}

// ConfigService devuelve la configuración estándar para el servicio del SO.
func ConfigService(runner Runner) (service.Service, error) {
	svcConfig := &service.Config{
		Name:        "CloudflareNetConnect",
		DisplayName: "Cloudflare NetConnect Multi-Account Service",
		Description: "Gestor multi-cuenta de túneles Cloudflare y conectividad Zero Trust.",
		Arguments:   []string{"run"},
	}

	prg := &program{runner: runner}
	return service.New(prg, svcConfig)
}

// ExecuteAction procesa comandos administrativos del servicio.
func ExecuteAction(s service.Service, action string) error {
	switch action {
	case "install":
		if err := s.Install(); err != nil {
			return fmt.Errorf("error al instalar servicio: %w", err)
		}
		log.Println("[OK] Servicio CloudflareNetConnect instalado con éxito.")
		return nil
	case "uninstall":
		if err := s.Uninstall(); err != nil {
			return fmt.Errorf("error al desinstalar servicio: %w", err)
		}
		log.Println("[OK] Servicio CloudflareNetConnect desinstalado con éxito.")
		return nil
	case "start":
		if err := s.Start(); err != nil {
			return fmt.Errorf("error al iniciar servicio: %w", err)
		}
		log.Println("[OK] Servicio iniciado.")
		return nil
	case "stop":
		if err := s.Stop(); err != nil {
			return fmt.Errorf("error al detener servicio: %w", err)
		}
		log.Println("[OK] Servicio detenido.")
		return nil
	case "restart":
		if err := s.Restart(); err != nil {
			return fmt.Errorf("error al reiniciar servicio: %w", err)
		}
		log.Println("[OK] Servicio reiniciado.")
		return nil
	case "status":
		st, err := s.Status()
		if err != nil {
			return fmt.Errorf("error al consultar estado: %w", err)
		}
		log.Printf("[STATUS] Estado actual del servicio: %v\n", st)
		return nil
	default:
		return fmt.Errorf("acción de servicio no reconocida: %s (use install, uninstall, start, stop, restart, status)", action)
	}
}
