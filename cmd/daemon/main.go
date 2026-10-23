package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/service"
	"github.com/chgarcia79/cloudflarenetconnect/internal/tray"
	"github.com/chgarcia79/cloudflarenetconnect/internal/tunnel"
)

func main() {
	exePath, err := os.Executable()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Dir(exePath)
	}

	logPath := config.DefaultLogPath(baseDir)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	defaultConfig := config.DefaultConfigPath(baseDir)
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

	if len(flag.Args()) > 0 && flag.Args()[0] == "run" {
		if err := svc.Run(); err != nil {
			log.Fatalf("[FATAL] Error en ejecución de servicio: %v", err)
		}
		return
	}

	log.Println("=== CloudflareNetConnect - Modo Bandeja del Sistema (Tray) ===")
	if err := orch.Start(); err != nil {
		log.Fatalf("[FATAL] Error al iniciar orquestador: %v", err)
	}
	trayApp := tray.NewTrayApp(orch, cfg, *configPath)
	trayApp.Run()
}
