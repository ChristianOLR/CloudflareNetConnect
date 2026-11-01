//go:build windows

package wireguard

import (
	"bytes"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

type WindowsTunnel struct {
	BaseTunnel
	dev      *device.Device
	tunDev   tun.Device
	luid     winipcfg.LUID
	destCIDR netip.Prefix
}

func NewTunnel(cfg *config.PrivateNetworkConfig) Tunnel {
	return &WindowsTunnel{
		BaseTunnel: BaseTunnel{cfg: cfg, status: StatusDisconnected},
	}
}

func (w *WindowsTunnel) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.status == StatusConnected {
		return nil
	}

	if err := EnsureWintunDLL(); err != nil {
		return fmt.Errorf("error cargando wintun: %w", err)
	}

	enrolled, err := EnsureEnrolled(w.cfg)
	if err != nil {
		return fmt.Errorf("error enrolando dispositivo: %w", err)
	}

	tunDev, err := tun.CreateTUN("CloudflareNetConnect", 1420)
	if err != nil {
		return fmt.Errorf("error creando interfaz wintun: %w", err)
	}
	w.tunDev = tunDev

	nativeTun, ok := tunDev.(*tun.NativeTun)
	if !ok {
		_ = tunDev.Close()
		return fmt.Errorf("tipo de interfaz nativa inválido")
	}
	w.luid = winipcfg.LUID(nativeTun.LUID())

	addrStr := strings.TrimSpace(enrolled.AddressIPv4)
	if !strings.Contains(addrStr, "/") {
		addrStr += "/32"
	}
	assignedPrefix, err := netip.ParsePrefix(addrStr)
	if err != nil {
		_ = tunDev.Close()
		return fmt.Errorf("prefijo IP inválido %s: %w", addrStr, err)
	}

	if err := w.luid.SetIPAddresses([]netip.Prefix{assignedPrefix}); err != nil {
		log.Printf("[WG-WARN] No se pudo asignar IP al adaptador (¿ejecutando como Admin?): %v", err)
	}

	cidrStr := strings.TrimSpace(w.cfg.CIDR)
	if cidrStr != "" {
		if destPrefix, err := netip.ParsePrefix(cidrStr); err == nil {
			w.destCIDR = destPrefix
			_ = w.luid.AddRoute(destPrefix, netip.IPv4Unspecified(), 0)
			log.Printf("[WG-ROUTE] Ruta privada inyectada: %s vía adaptador CloudflareNetConnect\n", cidrStr)
		}
	}

	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, "[WG] "))
	w.dev = dev

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
	w.status = StatusConnected
	log.Printf("[WG] ✅ Red Privada activa hacia %s (IP túnel: %s)\n", w.cfg.CIDR, assignedPrefix.Addr())
	return nil
}

func (w *WindowsTunnel) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.status == StatusDisconnected {
		return nil
	}

	if w.destCIDR.IsValid() {
		_ = w.luid.DeleteRoute(w.destCIDR, netip.IPv4Unspecified())
	}
	if w.dev != nil {
		w.dev.Close()
		w.dev = nil
	}
	if w.tunDev != nil {
		_ = w.tunDev.Close()
		w.tunDev = nil
	}

	w.status = StatusDisconnected
	log.Printf("[WG] 🔌 Red Privada desconectada limpiamente.\n")
	return nil
}

func (w *WindowsTunnel) Pause() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.status != StatusConnected {
		return nil
	}
	if w.destCIDR.IsValid() {
		_ = w.luid.DeleteRoute(w.destCIDR, netip.IPv4Unspecified())
	}
	w.status = StatusPausedLocal
	log.Printf("[WG] ⏸️ Red Privada pausada (Bypass automático para red local).\n")
	return nil
}

func (w *WindowsTunnel) Resume() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.status != StatusPausedLocal {
		return nil
	}
	if w.destCIDR.IsValid() {
		_ = w.luid.AddRoute(w.destCIDR, netip.IPv4Unspecified(), 0)
	}
	w.status = StatusConnected
	log.Printf("[WG] ▶️ Red Privada reanudada (Conexión remota activa).\n")
	return nil
}
