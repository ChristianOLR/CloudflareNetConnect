//go:build windows

package netmonitor

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi          = windows.NewLazySystemDLL("iphlpapi.dll")
	procNotifyAddrChange = modiphlpapi.NewProc("NotifyAddrChange")
)

// StartWatcher inicia el oyente de eventos de red del kernel de Windows sin consumo de CPU.
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
		for {
			select {
			case <-d.stopCh:
				return
			default:
			}

			var handle windows.Handle
			var overlapped windows.Overlapped
			event, err := windows.CreateEvent(nil, 0, 0, nil)
			if err == nil && event != 0 {
				overlapped.HEvent = event
				r, _, _ := procNotifyAddrChange.Call(uintptr(unsafe.Pointer(&handle)), uintptr(unsafe.Pointer(&overlapped)))
				if r == 997 { // ERROR_IO_PENDING
					_, _ = windows.WaitForSingleObject(event, 5000)
				}
				_ = windows.CloseHandle(event)
			} else {
				time.Sleep(5 * time.Second)
			}

			select {
			case <-d.stopCh:
				return
			default:
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
