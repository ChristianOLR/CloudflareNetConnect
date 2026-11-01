//go:build windows

package ui

import (
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func CopyToClipboard(text string) {
	_ = CopyToClipboardWithHwnd(0, text)
}

func CopyToClipboardWithHwnd(hWnd windows.Handle, text string) bool {
	if text == "" {
		return false
	}
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")

	for i := 0; i < 20; i++ {
		r, _, _ := u32.NewProc("OpenClipboard").Call(uintptr(hWnd))
		if r != 0 {
			defer u32.NewProc("CloseClipboard").Call()
			u32.NewProc("EmptyClipboard").Call()

			utf16, _ := windows.UTF16FromString(text)
			hMem, _, _ := k32.NewProc("GlobalAlloc").Call(0x0042, uintptr(len(utf16)*2))
			if hMem != 0 {
				ptr, _, _ := k32.NewProc("GlobalLock").Call(hMem)
				if ptr != 0 {
					dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
					copy(dst, utf16)
					k32.NewProc("GlobalUnlock").Call(hMem)
					if setRes, _, _ := u32.NewProc("SetClipboardData").Call(13, hMem); setRes != 0 {
						return true
					}
				}
			}
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Set-Clipboard", "-Value", text)
	return cmd.Run() == nil
}

// ReadClipboardText lee texto en formato UTF-16 del portapapeles.
func ReadClipboardText() string {
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	for i := 0; i < 5; i++ {
		r, _, _ := u32.NewProc("OpenClipboard").Call(0)
		if r != 0 {
			defer u32.NewProc("CloseClipboard").Call()
			hData, _, _ := u32.NewProc("GetClipboardData").Call(13) // CF_UNICODETEXT
			if hData == 0 {
				return ""
			}
			ptr, _, _ := k32.NewProc("GlobalLock").Call(hData)
			if ptr == 0 {
				return ""
			}
			defer k32.NewProc("GlobalUnlock").Call(hData)
			return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr)))
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ""
}
