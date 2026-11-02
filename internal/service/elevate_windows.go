//go:build windows

package service

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// EnsureAdmin verifica si el proceso corre con privilegios elevados en Windows.
// Si no es admin, lanza una nueva instancia pidiendo elevación UAC y retorna true
// indicando que el proceso llamador debe salir.
func EnsureAdmin() bool {
	token := windows.Token(0)
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err == nil {
		defer token.Close()
		var elevated uint32
		var returnedLen uint32
		err = windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &returnedLen)
		if err == nil && elevated != 0 {
			return false // Ya es administrador
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return false
	}

	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(filepath.Dir(exe))

	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	argsStr := strings.Join(args, " ")
	argsPtr, _ := syscall.UTF16PtrFromString(argsStr)

	showCmd := int32(windows.SW_SHOWNORMAL)
	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, showCmd)
	if err == nil {
		log.Println("[ELEVATE] Relanzando CloudflareNetConnect con privilegios de Administrador...")
		return true
	}
	log.Printf("[ELEVATE-WARN] No se pudo solicitar elevación UAC: %v\n", err)
	return false
}
