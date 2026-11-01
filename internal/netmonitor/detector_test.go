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
	// Allow any asynchronous callback execution
	time.Sleep(10 * time.Millisecond)

	d.StartWatcher()
	d.StopWatcher()
}
