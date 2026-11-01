//go:build windows

package wireguard

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

//go:embed assets/wintun_amd64.dll
var wintunAmd64 []byte

//go:embed assets/wintun_arm64.dll
var wintunArm64 []byte

var (
	extractOnce sync.Once
	extractErr  error
)

// EnsureWintunDLL extrae la DLL adecuada de wintun si no existe en la ruta de ejecución.
func EnsureWintunDLL() error {
	extractOnce.Do(func() {
		dllBytes := wintunAmd64
		if runtime.GOARCH == "arm64" {
			dllBytes = wintunArm64
		}

		exePath, err := os.Executable()
		targetDir := "."
		if err == nil {
			targetDir = filepath.Dir(exePath)
		}
		targetFile := filepath.Join(targetDir, "wintun.dll")

		if _, err := os.Stat(targetFile); err == nil {
			_ = windows.SetDllDirectory(targetDir)
			return
		}

		err = os.WriteFile(targetFile, dllBytes, 0644)
		if err != nil {
			tempDir := os.TempDir()
			targetFile = filepath.Join(tempDir, "wintun.dll")
			err = os.WriteFile(targetFile, dllBytes, 0644)
			if err != nil {
				extractErr = err
				return
			}
			_ = windows.SetDllDirectory(tempDir)
			return
		}

		_ = windows.SetDllDirectory(targetDir)
	})
	return extractErr
}
