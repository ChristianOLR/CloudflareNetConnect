package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

	// Si se ejecuta como servicio en segundo plano (Session 0)
	if len(flag.Args()) > 0 && flag.Args()[0] == "run" {
		if err := svc.Run(); err != nil {
			log.Fatalf("[FATAL] Error en ejecución de servicio: %v", err)
		}
		return
	}

	// Modo interactivo con bandeja del sistema (System Tray nativo)
	fmt.Println("=== CloudflareNetConnect - Modo Bandeja del Sistema (Tray) ===")
	if err := orch.Start(); err != nil {
		log.Fatalf("[FATAL] Error al iniciar orquestador: %v", err)
	}
	trayApp := tray.NewTrayApp(orch, cfg, *configPath)
	trayApp.Run()
}
