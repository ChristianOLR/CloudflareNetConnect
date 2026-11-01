package wireguard

import (
	"encoding/base64"
	"encoding/hex"
	"net"
	"strings"
	"sync"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

// TunnelStatus representa el estado operativo de la conexión WireGuard.
type TunnelStatus int

const (
	StatusDisconnected TunnelStatus = iota
	StatusConnecting
	StatusConnected
	StatusPausedLocal
)

// Tunnel define la interfaz universal de control para la red privada.
type Tunnel interface {
	Start() error
	Stop() error
	Pause() error
	Resume() error
	Rebind() error
	Status() TunnelStatus
	CIDR() string
	VirtualCIDR() string
	SetVirtualCIDR(vCIDR string)
}

// BaseTunnel encapsula estado común de sincronización y soporte NAT.
type BaseTunnel struct {
	mu          sync.RWMutex
	cfg         *config.PrivateNetworkConfig
	status      TunnelStatus
	virtualCIDR string
	nat         *NATTranslator
}

func (b *BaseTunnel) Status() TunnelStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

func (b *BaseTunnel) CIDR() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.cfg == nil {
		return ""
	}
	return b.cfg.CIDR
}

func (b *BaseTunnel) VirtualCIDR() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.virtualCIDR
}

func (b *BaseTunnel) SetVirtualCIDR(vCIDR string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.virtualCIDR = strings.TrimSpace(vCIDR)
	if b.virtualCIDR != "" && b.cfg != nil && b.cfg.CIDR != "" {
		if trans, err := NewNATTranslator(b.cfg.CIDR, b.virtualCIDR); err == nil {
			b.nat = trans
		}
	} else {
		b.nat = nil
	}
}

func (b *BaseTunnel) SetStatus(s TunnelStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = s
}

func toHexKey(b64Key string) string {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Key))
	if err != nil || len(b) != 32 {
		return ""
	}
	return hex.EncodeToString(b)
}

func resolvePeerEndpoint(endpoint string) string {
	ep := normalizeEndpointAddr(endpoint, 2408)
	if ep == "" {
		ep = cfDefaultPeer
	}
	addr, err := net.ResolveUDPAddr("udp", ep)
	if err == nil && addr != nil {
		return addr.String()
	}
	if strings.HasPrefix(ep, "162.159.193.") {
		return cfZeroTrustDefaultPeer
	}
	return "162.159.192.1:2408"
}
