//go:build !windows && !darwin

package wireguard

import (
	"fmt"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

type StubTunnel struct {
	BaseTunnel
}

func NewTunnel(cfg *config.PrivateNetworkConfig) Tunnel {
	return &StubTunnel{
		BaseTunnel: BaseTunnel{cfg: cfg, status: StatusDisconnected},
	}
}

func (s *StubTunnel) Start() error {
	return fmt.Errorf("red privada no soportada en esta plataforma")
}

func (s *StubTunnel) Stop() error {
	s.status = StatusDisconnected
	return nil
}

func (s *StubTunnel) Pause() error {
	s.status = StatusPausedLocal
	return nil
}

func (s *StubTunnel) Resume() error {
	s.status = StatusConnected
	return nil
}
