//go:build windows

package tray

import (
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// isEffectivelyActive devuelve true solo si el Interruptor Maestro está en ON y hay al menos una cuenta activa.
func (t *TrayApp) isEffectivelyActive() bool {
	if !t.cfg.MasterToggle {
		return false
	}
	for _, acc := range t.cfg.Accounts {
		if acc.Enabled {
			return true
		}
	}
	return false
}

// refreshTrayState sincroniza el icono del System Tray y su tooltip según el estado efectivo real.
func (t *TrayApp) refreshTrayState() {
	t.mu.Lock()
	active := t.isEffectivelyActive()
	hasAccounts := len(t.cfg.Accounts) > 0
	masterOn := t.cfg.MasterToggle
	t.mu.Unlock()

	s32 := windows.NewLazySystemDLL("shell32.dll")
	pShellNotifyIcon := s32.NewProc("Shell_NotifyIconW")

	t.hIcon = t.createHIcon(active)
	t.nid.HIcon = t.hIcon

	tipText := "CloudflareNetConnect - En Reposo (Sin cuentas activas)"
	if active {
		tipText = "CloudflareNetConnect - Activo (ON)"
	} else if !masterOn {
		tipText = "CloudflareNetConnect - Pausado (Interruptor OFF)"
	} else if hasAccounts {
		tipText = "CloudflareNetConnect - En Reposo (Dominios pausados)"
	}

	tip, _ := windows.UTF16FromString(tipText)
	for i := range t.nid.SzTip {
		t.nid.SzTip[i] = 0
	}
	copy(t.nid.SzTip[:], tip)

	pShellNotifyIcon.Call(1, uintptr(unsafe.Pointer(&t.nid))) // NIM_MODIFY = 1
}

// createColorDotBitmap genera un HBITMAP 16x16 de 32-bit ARGB con una bolita antialias verde o roja.
func createColorDotBitmap(isGreen bool) windows.Handle {
	gdi := windows.NewLazySystemDLL("gdi32.dll")
	pCreateDIBSection := gdi.NewProc("CreateDIBSection")

	const size = 16
	bmi := bitmapInfoHeader{
		BiSize:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		BiWidth:    size,
		BiHeight:   -size, // top-down
		BiPlanes:   1,
		BiBitCount: 32,
	}

	var bitsPtr uintptr
	hbm, _, _ := pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bitsPtr)), 0, 0)
	if hbm == 0 || bitsPtr == 0 {
		return 0
	}

	pixels := unsafe.Slice((*uint32)(unsafe.Pointer(bitsPtr)), size*size)

	// Colores en formato 0xAARRGGBB (pre-multiplicado para AlphaBlend de menús Win32)
	var baseR, baseG, baseB float64 = 239, 68, 68 // Rojo #EF4444
	if isGreen {
		baseR, baseG, baseB = 16, 185, 129 // Verde #10B981
	}

	cx, cy := 7.5, 7.5
	radius := 5.2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			var alpha float64
			if dist <= radius-0.6 {
				alpha = 1.0
			} else if dist <= radius+0.6 {
				alpha = (radius + 0.6 - dist) / 1.2
			} else {
				alpha = 0.0
			}

			if alpha > 0 {
				a := uint32(alpha * 255)
				r := uint32(baseR * alpha)
				g := uint32(baseG * alpha)
				b := uint32(baseB * alpha)
				pixels[y*size+x] = (a << 24) | (r << 16) | (g << 8) | b
			} else {
				pixels[y*size+x] = 0
			}
		}
	}

	return windows.Handle(hbm)
}

// isStartupEnabled verifica si el servicio está registrado en el autoarranque del usuario (HKCU\...\Run).
func isStartupEnabled() bool {
	adv := windows.NewLazySystemDLL("advapi32.dll")
	var hKey windows.Handle
	subKey, _ := windows.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Run`)
	ret, _, _ := adv.NewProc("RegOpenKeyExW").Call(0x80000001, uintptr(unsafe.Pointer(subKey)), 0, 0x20019, uintptr(unsafe.Pointer(&hKey)))
	if ret != 0 {
		return false
	}
	defer adv.NewProc("RegCloseKey").Call(uintptr(hKey))

	valName, _ := windows.UTF16PtrFromString("CloudflareNetConnect")
	var dataLen uint32
	r2, _, _ := adv.NewProc("RegQueryValueExW").Call(uintptr(hKey), uintptr(unsafe.Pointer(valName)), 0, 0, 0, uintptr(unsafe.Pointer(&dataLen)))
	return r2 == 0 && dataLen > 0
}

// setStartupEnabled activa o desactiva el autoarranque en Windows sin requerir permisos UAC.
func setStartupEnabled(enable bool, exePath string) {
	adv := windows.NewLazySystemDLL("advapi32.dll")
	var hKey windows.Handle
	subKey, _ := windows.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Run`)
	ret, _, _ := adv.NewProc("RegOpenKeyExW").Call(0x80000001, uintptr(unsafe.Pointer(subKey)), 0, 0x20006, uintptr(unsafe.Pointer(&hKey)))
	if ret != 0 {
		return
	}
	defer adv.NewProc("RegCloseKey").Call(uintptr(hKey))

	valName, _ := windows.UTF16PtrFromString("CloudflareNetConnect")
	if !enable {
		adv.NewProc("RegDeleteValueW").Call(uintptr(hKey), uintptr(unsafe.Pointer(valName)))
		return
	}
	quoted := `"` + exePath + `"`
	u16, _ := windows.UTF16FromString(quoted)
	adv.NewProc("RegSetValueExW").Call(uintptr(hKey), uintptr(unsafe.Pointer(valName)), 0, 1, uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)*2))
}

// runElevatedServiceCmd ejecuta -service install|uninstall|start|stop solicitando elevación UAC (runas).
func runElevatedServiceCmd(exePath, action string) {
	s32 := windows.NewLazySystemDLL("shell32.dll")
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exePath)
	args, _ := windows.UTF16PtrFromString("-service " + action)
	s32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(args)), 0, 0)
}
