package tunnel

import (
	"log"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/netmonitor"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

func (o *Orchestrator) startPrivateNetworkLocked(acc config.AccountConfig) {
	if acc.PrivateNetwork == nil || !acc.PrivateNetwork.Enabled {
		return
	}

	tun, exists := o.wgTunnels[acc.ID]
	if !exists {
		tun = wireguard.NewTunnel(acc.PrivateNetwork)
		o.wgTunnels[acc.ID] = tun
	}

	if acc.PrivateNetwork.AutoDetect {
		detector, detExists := o.netDetectors[acc.ID]
		if !detExists {
			probeIP := acc.PrivateNetwork.LocalProbeIP
			detector = netmonitor.NewDetector(acc.PrivateNetwork.CIDR, probeIP, func(inLocal bool) {
				if inLocal {
					_ = tun.Pause()
				} else {
					_ = tun.Resume()
				}
			})
			o.netDetectors[acc.ID] = detector
			detector.StartWatcher()
		}

		if detector.InLocal() {
			_ = tun.Start()
			_ = tun.Pause()
			return
		}
	}

	if err := tun.Start(); err != nil {
		log.Printf("[ORCH-WG-ERR:%s] Error iniciando red privada: %v\n", acc.ID, err)
	}
}

func (o *Orchestrator) stopPrivateNetworkLocked(accountID string) {
	if det, exists := o.netDetectors[accountID]; exists {
		det.StopWatcher()
		delete(o.netDetectors, accountID)
	}
	if tun, exists := o.wgTunnels[accountID]; exists {
		_ = tun.Stop()
		delete(o.wgTunnels, accountID)
	}
}

func (o *Orchestrator) stopAllPrivateNetworksLocked() {
	for id := range o.netDetectors {
		o.netDetectors[id].StopWatcher()
	}
	o.netDetectors = make(map[string]*netmonitor.NetworkDetector)

	for id := range o.wgTunnels {
		_ = o.wgTunnels[id].Stop()
	}
	o.wgTunnels = make(map[string]wireguard.Tunnel)
}

// TogglePrivateNetwork conmuta el estado de la Red Privada de una cuenta.
func (o *Orchestrator) TogglePrivateNetwork(accountID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.cfg.Accounts {
		if o.cfg.Accounts[i].ID == accountID {
			acc := &o.cfg.Accounts[i]
			if acc.PrivateNetwork == nil {
				acc.PrivateNetwork = &config.PrivateNetworkConfig{
					Enabled:    true,
					AutoDetect: true,
					CIDR:       "192.168.1.0/24",
				}
			} else {
				acc.PrivateNetwork.Enabled = !acc.PrivateNetwork.Enabled
			}

			if acc.PrivateNetwork.Enabled && o.active && o.cfg.MasterToggle {
				o.startPrivateNetworkLocked(*acc)
			} else {
				o.stopPrivateNetworkLocked(accountID)
			}
			return acc.PrivateNetwork.Enabled
		}
	}
	return false
}

// TogglePrivateNetworkAutoDetect conmuta el auto-bypass local para la Red Privada.
func (o *Orchestrator) TogglePrivateNetworkAutoDetect(accountID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.cfg.Accounts {
		if o.cfg.Accounts[i].ID == accountID {
			acc := &o.cfg.Accounts[i]
			if acc.PrivateNetwork == nil {
				acc.PrivateNetwork = &config.PrivateNetworkConfig{
					Enabled:    false,
					AutoDetect: true,
					CIDR:       "192.168.1.0/24",
				}
			} else {
				acc.PrivateNetwork.AutoDetect = !acc.PrivateNetwork.AutoDetect
			}

			if acc.PrivateNetwork.Enabled && o.active && o.cfg.MasterToggle {
				o.stopPrivateNetworkLocked(accountID)
				o.startPrivateNetworkLocked(*acc)
			}
			return acc.PrivateNetwork.AutoDetect
		}
	}
	return false
}

// GetPrivateNetworkInfo devuelve el estado actual de la red privada para el menú.
func (o *Orchestrator) GetPrivateNetworkInfo(accountID string) (status wireguard.TunnelStatus, cidr string, autoDetect bool, enabled bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, acc := range o.cfg.Accounts {
		if acc.ID == accountID {
			if acc.PrivateNetwork == nil {
				return wireguard.StatusDisconnected, "", false, false
			}
			cidr = acc.PrivateNetwork.CIDR
			autoDetect = acc.PrivateNetwork.AutoDetect
			enabled = acc.PrivateNetwork.Enabled
			if tun, exists := o.wgTunnels[accountID]; exists {
				status = tun.Status()
			} else if enabled {
				status = wireguard.StatusConnecting
			} else {
				status = wireguard.StatusDisconnected
			}
			return status, cidr, autoDetect, enabled
		}
	}
	return wireguard.StatusDisconnected, "", false, false
}
