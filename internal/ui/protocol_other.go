//go:build !windows

package ui

// RegisterWarpProtocol es un stub no operativo en plataformas no-Windows.
func RegisterWarpProtocol() error {
	return nil
}
