//go:build windows

package netmonitor

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi              = windows.NewLazySystemDLL("iphlpapi.dll")
	procNotifyAddrChange     = modiphlpapi.NewProc("NotifyAddrChange")
	procCancelIPChangeNotify = modiphlpapi.NewProc("CancelIPChangeNotify")
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
			addrChanged := false
			event, err := windows.CreateEvent(nil, 0, 0, nil)
			if err == nil && event != 0 {
				overlapped.HEvent = event
				r, _, _ := procNotifyAddrChange.Call(uintptr(unsafe.Pointer(&handle)), uintptr(unsafe.Pointer(&overlapped)))
				if r == 997 { // ERROR_IO_PENDING
					waitRes, _ := windows.WaitForSingleObject(event, 2500)
					if waitRes == windows.WAIT_OBJECT_0 {
						addrChanged = true
					} else {
						_, _, _ = procCancelIPChangeNotify.Call(uintptr(unsafe.Pointer(&overlapped)))
					}
				} else if r == 0 {
					addrChanged = true
				}
				_ = windows.CloseHandle(event)
			} else {
				time.Sleep(2500 * time.Millisecond)
			}

			select {
			case <-d.stopCh:
				return
			default:
				if addrChanged {
					time.Sleep(350 * time.Millisecond)
					d.ForceCheck()
				} else {
					d.CheckNow()
				}
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
