package tunnel

import (
	"fmt"
	"log"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
	"github.com/chgarcia79/cloudflarenetconnect/internal/netmonitor"
	"github.com/chgarcia79/cloudflarenetconnect/internal/wireguard"
)

// SetOnStateChange registra un callback ejecutado cuando cambia el estado de conexión o red.
func (o *Orchestrator) SetOnStateChange(cb func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.onStateChange = cb
}

func (o *Orchestrator) notifyStateChangeLocked() {
	if o.onStateChange != nil {
		cb := o.onStateChange
		go cb()
	}
}

// reconcilePrivateNetwork reevalúa la topología ante cualquier evento de cambio de red.
func (o *Orchestrator) reconcilePrivateNetwork(accountID string, inLocal bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	tun, exists := o.wgTunnels[accountID]
	if !exists {
		return
	}

	var targetAcc *config.AccountConfig
	for i := range o.cfg.Accounts {
		if o.cfg.Accounts[i].ID == accountID {
			targetAcc = &o.cfg.Accounts[i]
			break
		}
	}
	if targetAcc == nil || targetAcc.PrivateNetwork == nil || !targetAcc.PrivateNetwork.Enabled {
		return
	}

	if inLocal && targetAcc.PrivateNetwork.AutoDetect {
		_ = tun.Pause()
		log.Printf("[ORCH-RECONCILE:%s] 🏠 Red local detectada -> Pausa LAN activa\n", accountID)
		o.notifyStateChangeLocked()
		return
	}

	neededVirtCIDR := o.resolveVirtualCIDR(accountID, targetAcc.PrivateNetwork.CIDR)
	currentVirtCIDR := tun.VirtualCIDR()

	if neededVirtCIDR != currentVirtCIDR {
		log.Printf("[ORCH-RECONCILE:%s] 🔄 Cambio de topología: VirtualCIDR era '%s', conmutando a '%s'\n",
			accountID, currentVirtCIDR, neededVirtCIDR)
		_ = tun.Stop()
		tun.SetVirtualCIDR(neededVirtCIDR)
		if err := tun.Start(); err != nil {
			log.Printf("[ORCH-RECONCILE:%s] Error reconectando túnel: %v\n", accountID, err)
		}
	} else if tun.Status() == wireguard.StatusPausedLocal {
		_ = tun.Resume()
		log.Printf("[ORCH-RECONCILE:%s] 🟢 Reanudando túnel remoto tras salir de LAN local\n", accountID)
	}

	o.notifyStateChangeLocked()
}

// resolveVirtualCIDR calcula si existe colisión y asigna una subred virtual no conflictiva.
func (o *Orchestrator) resolveVirtualCIDR(accountID, targetCIDR string) string {
	hasLocalConflict := netmonitor.HasLocalPhysicalSubnet(targetCIDR)
	hasRemoteConflict := false

	for id, otherTun := range o.wgTunnels {
		if id != accountID && otherTun.Status() != wireguard.StatusDisconnected {
			if otherTun.CIDR() == targetCIDR || otherTun.VirtualCIDR() == targetCIDR {
				hasRemoteConflict = true
				break
			}
		}
	}

	if !hasLocalConflict && !hasRemoteConflict {
		return ""
	}

	for i := 1; i <= 254; i++ {
		candidate := fmt.Sprintf("10.200.%d.0/24", i)
		inUse := netmonitor.HasLocalPhysicalSubnet(candidate)
		if inUse {
			continue
		}
		for _, t := range o.wgTunnels {
			if t.VirtualCIDR() == candidate {
				inUse = true
				break
			}
		}
		if !inUse {
			return candidate
		}
	}
	return "10.200.1.0/24"
}
