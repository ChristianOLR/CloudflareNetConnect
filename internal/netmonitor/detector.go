package netmonitor

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// NetworkDetector detecta si el equipo está conectado directamente a la LAN local o en red externa.
type NetworkDetector struct {
	mu           sync.RWMutex
	inLocal      bool
	targetCIDR   string
	probeIP      string
	onChange     func(inLocal bool)
	stopCh       chan struct{}
	started      bool
}

// NewDetector crea una nueva instancia de monitoreo de red.
func NewDetector(targetCIDR, probeIP string, onChange func(inLocal bool)) *NetworkDetector {
	return &NetworkDetector{
		targetCIDR: targetCIDR,
		probeIP:    probeIP,
		onChange:   onChange,
		stopCh:     make(chan struct{}),
	}
}

// InLocal devuelve el estado actual de ubicación de red.
func (d *NetworkDetector) InLocal() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.inLocal
}

// CheckNow evalúa de inmediato si la máquina se encuentra en la subred local física.
func (d *NetworkDetector) CheckNow() bool {
	isLocal := d.probeNetwork()

	d.mu.Lock()
	changed := isLocal != d.inLocal
	d.inLocal = isLocal
	callback := d.onChange
	d.mu.Unlock()

	if changed {
		if isLocal {
			log.Printf("[NET-DETECT] 🏠 Red local detectada (Conexión LAN física activa).\n")
		} else {
			log.Printf("[NET-DETECT] 🌍 Red externa detectada (Fuera de la LAN local).\n")
		}
		if callback != nil {
			go callback(isLocal)
		}
	}
	return isLocal
}

func (d *NetworkDetector) probeNetwork() bool {
	targetCIDR := strings.TrimSpace(d.targetCIDR)
	if targetCIDR == "" {
		return false
	}

	_, targetNet, err := net.ParseCIDR(targetCIDR)
	if err != nil {
		return false
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	hasLocalIP := false
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Ignorar interfaces de túneles virtuales
		name := strings.ToLower(iface.Name)
		if strings.Contains(name, "wintun") || strings.Contains(name, "cloudflare") || strings.Contains(name, "utun") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			if targetNet.Contains(ip) {
				hasLocalIP = true
				break
			}
		}
		if hasLocalIP {
			break
		}
	}

	if !hasLocalIP {
		return false
	}

	// Si hay IP local y se definió una IP de sonda (ej. router 192.168.1.1), verificar respuesta rápida TCP
	probe := strings.TrimSpace(d.probeIP)
	if probe != "" {
		hostPort := probe
		if !strings.Contains(hostPort, ":") {
			hostPort += ":80"
		}
		conn, err := net.DialTimeout("tcp", hostPort, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}

	return true
}
