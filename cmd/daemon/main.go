package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/olrsoft/cloudflarenetconnect/internal/config"
	"github.com/olrsoft/cloudflarenetconnect/internal/service"
	"github.com/olrsoft/cloudflarenetconnect/internal/tray"
	"github.com/olrsoft/cloudflarenetconnect/internal/tunnel"
)

func main() {
	exePath, err := os.Executable()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Dir(exePath)
	}
	defaultConfig := filepath.Join(baseDir, "config", "netconnect.json")

	configPath := flag.String("config", defaultConfig, "Ruta al config")
	serviceAction := flag.String("service", "", "Acción: install, uninstall, start, stop, restart, status")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Error cargando configuración: %v", err)
	}

	orch := tunnel.NewOrchestrator(cfg)
	_ = tray.NewTrayHandler(orch)

	svc, err := service.ConfigService(orch)
	if err != nil {
		log.Fatalf("[FATAL] Error configurando servicio: %v", err)
	}

	if *serviceAction != "" {
		if err := service.ExecuteAction(svc, *serviceAction); err != nil {
			log.Fatalf("[FATAL] %v", err)
		}
		return
	}

	if len(flag.Args()) > 0 && flag.Args()[0] == "run" {
		if err := svc.Run(); err != nil {
			log.Fatalf("[FATAL] Error en ejecución de servicio: %v", err)
		}
		return
	}

	fmt.Println("=== CloudflareNetConnect - Multi-Account Daemon ===")
	if err := orch.Start(); err != nil {
		log.Fatalf("[FATAL] Error al iniciar orquestador: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\n[INFO] Señal recibida. Cerrando conexiones...")
	_ = orch.Stop()
	log.Println("[INFO] Salida limpia completada.")
}
