package wireguard

import (
	"encoding/base64"
	"encoding/hex"
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
	Status() TunnelStatus
	CIDR() string
}

// BaseTunnel encapsula estado común de sincronización.
type BaseTunnel struct {
	mu     sync.RWMutex
	cfg    *config.PrivateNetworkConfig
	status TunnelStatus
}

func (b *BaseTunnel) Status() TunnelStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

func (b *BaseTunnel) CIDR() string {
	if b.cfg == nil {
		return ""
	}
	return b.cfg.CIDR
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
