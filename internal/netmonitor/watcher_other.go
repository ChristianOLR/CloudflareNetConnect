//go:build !windows

package netmonitor

import (
	"time"
)

// StartWatcher inicia el oyente de eventos de red en plataformas no Windows.
func (d *NetworkDetector) StartWatcher() {
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return
	}
	d.started = true
	d.mu.Unlock()

	d.CheckNow()

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCh:
				return
			case <-ticker.C:
				d.CheckNow()
			}
		}
	}()
}

// StopWatcher detiene la escucha de eventos de red.
func (d *NetworkDetector) StopWatcher() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started {
		return
	}
	d.started = false
	close(d.stopCh)
}
