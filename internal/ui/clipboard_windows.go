//go:build windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func CopyToClipboard(text string) {
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	r, _, _ := u32.NewProc("OpenClipboard").Call(0)
	if r == 0 {
		return
	}
	defer u32.NewProc("CloseClipboard").Call()
	u32.NewProc("EmptyClipboard").Call()

	utf16, _ := windows.UTF16FromString(text)
	hMem, _, _ := k32.NewProc("GlobalAlloc").Call(0x0042, uintptr(len(utf16)*2))
	if hMem == 0 {
		return
	}
	ptr, _, _ := k32.NewProc("GlobalLock").Call(hMem)
	if ptr != 0 {
		dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
		copy(dst, utf16)
		k32.NewProc("GlobalUnlock").Call(hMem)
		u32.NewProc("SetClipboardData").Call(13, hMem)
	}
}
