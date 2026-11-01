package netmonitor

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDetectorInitialization(t *testing.T) {
	var notified atomic.Bool
	d := NewDetector("10.0.0.0/24", "10.0.0.1", func(inLocal bool) {
		notified.Store(true)
	})

	if d.InLocal() {
		t.Errorf("expected InLocal() to be false initially")
	}

	d.CheckNow()
	time.Sleep(10 * time.Millisecond)

	d.StartWatcher()
	d.StopWatcher()
}

func TestHasLocalPhysicalSubnet(t *testing.T) {
	// Comprobar que no entra en pánico con CIDR válido o inválido
	_ = HasLocalPhysicalSubnet("127.0.0.0/8")
	_ = HasLocalPhysicalSubnet("invalid-cidr")
	_ = HasLocalPhysicalSubnet("")
}
