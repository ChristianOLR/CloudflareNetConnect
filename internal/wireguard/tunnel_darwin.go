//go:build darwin

package wireguard

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

type DarwinTunnel struct {
	BaseTunnel
	dev    *device.Device
	tunDev tun.Device
	ifName string
}

func NewTunnel(cfg *config.PrivateNetworkConfig) Tunnel {
	return &DarwinTunnel{
		BaseTunnel: BaseTunnel{cfg: cfg, status: StatusDisconnected},
	}
}

func (d *DarwinTunnel) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status == StatusConnected {
		return nil
	}

	enrolled, err := EnsureEnrolled(d.cfg)
	if err != nil {
		return fmt.Errorf("error enrolando dispositivo: %w", err)
	}

	tunDev, err := tun.CreateTUN("utun", 1420)
	if err != nil {
		return fmt.Errorf("error creando interfaz utun nativa: %w", err)
	}
	d.tunDev = tunDev

	ifName, err := tunDev.Name()
	if err != nil {
		_ = tunDev.Close()
		return fmt.Errorf("error obteniendo nombre utun: %w", err)
	}
	d.ifName = ifName

	addr := strings.Split(enrolled.AddressIPv4, "/")[0]
	_ = exec.Command("ifconfig", ifName, addr, addr, "up").Run()

	if d.cfg.CIDR != "" {
		_ = exec.Command("route", "add", "-net", d.cfg.CIDR, "-interface", ifName).Run()
		log.Printf("[WG:macOS] Ruta privada inyectada: %s vía %s\n", d.cfg.CIDR, ifName)
	}

	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, "[WG] "))
	d.dev = dev

	privHex := toHexKey(enrolled.PrivateKey)
	peerHex := toHexKey(enrolled.PeerPubKey)
	peerEndpoint := enrolled.PeerEndpoint
	if peerEndpoint == "" {
		peerEndpoint = cfDefaultPeer
	}

	ipcConfig := fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=0.0.0.0/0\npersistent_keepalive_interval=25\n",
		privHex, peerHex, peerEndpoint)
	if err := dev.IpcSetOperation(bytes.NewBufferString(ipcConfig)); err != nil {
		dev.Close()
		_ = tunDev.Close()
		return fmt.Errorf("error aplicando configuración IPC WireGuard: %w", err)
	}

	_ = dev.Up()
	d.status = StatusConnected
	log.Printf("[WG:macOS] ✅ Red Privada activa hacia %s vía %s\n", d.cfg.CIDR, ifName)
	return nil
}

func (d *DarwinTunnel) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status == StatusDisconnected {
		return nil
	}

	if d.cfg.CIDR != "" && d.ifName != "" {
		_ = exec.Command("route", "delete", "-net", d.cfg.CIDR, "-interface", d.ifName).Run()
	}
	if d.dev != nil {
		d.dev.Close()
		d.dev = nil
	}
	if d.tunDev != nil {
		_ = d.tunDev.Close()
		d.tunDev = nil
	}

	d.status = StatusDisconnected
	log.Printf("[WG:macOS] 🔌 Red Privada desconectada limpiamente.\n")
	return nil
}

func (d *DarwinTunnel) Pause() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status != StatusConnected {
		return nil
	}
	if d.cfg.CIDR != "" && d.ifName != "" {
		_ = exec.Command("route", "delete", "-net", d.cfg.CIDR, "-interface", d.ifName).Run()
	}
	d.status = StatusPausedLocal
	log.Printf("[WG:macOS] ⏸️ Red Privada pausada (Bypass automático para red local).\n")
	return nil
}

func (d *DarwinTunnel) Resume() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status != StatusPausedLocal {
		return nil
	}
	if d.cfg.CIDR != "" && d.ifName != "" {
		_ = exec.Command("route", "add", "-net", d.cfg.CIDR, "-interface", d.ifName).Run()
	}
	d.status = StatusConnected
	log.Printf("[WG:macOS] ▶️ Red Privada reanudada (Conexión remota activa).\n")
	return nil
}
