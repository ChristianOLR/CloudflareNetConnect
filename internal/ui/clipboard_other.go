//go:build !windows

package ui

// ReadClipboardText es un stub no operativo en plataformas no-Windows.
func ReadClipboardText() string {
	return ""
}
