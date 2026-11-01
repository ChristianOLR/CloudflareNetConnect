//go:build windows

package ui

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// RegisterWarpProtocol registra en HKCU el manejador de protocolo com.cloudflare.warp://
func RegisterWarpProtocol() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\com.cloudflare.warp`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()

	_ = key.SetStringValue("", "URL:Cloudflare WARP Protocol")
	_ = key.SetStringValue("URL Protocol", "")

	cmdKey, _, err := registry.CreateKey(key, `shell\open\command`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer cmdKey.Close()

	cmdVal := fmt.Sprintf(`"%s" "%%1"`, exePath)
	return cmdKey.SetStringValue("", cmdVal)
}
