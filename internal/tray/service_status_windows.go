//go:build windows

package tray

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type SvcState int

const (
	SvcNotInstalled SvcState = 0
	SvcStopped      SvcState = 1
	SvcRunning      SvcState = 4
	SvcOther        SvcState = 9
)

// queryWindowsServiceStatus consulta el estado real del servicio Windows mediante el SCM de Win32.
func queryWindowsServiceStatus() SvcState {
	adv := windows.NewLazySystemDLL("advapi32.dll")
	pOpenSCManager := adv.NewProc("OpenSCManagerW")
	pOpenService := adv.NewProc("OpenServiceW")
	pQueryServiceStatus := adv.NewProc("QueryServiceStatus")
	pCloseServiceHandle := adv.NewProc("CloseServiceHandle")

	hSCM, _, _ := pOpenSCManager.Call(0, 0, 0x0001) // SC_MANAGER_CONNECT
	if hSCM == 0 {
		return SvcNotInstalled
	}
	defer pCloseServiceHandle.Call(hSCM)

	svcName, _ := windows.UTF16PtrFromString("CloudflareNetConnect")
	hSvc, _, _ := pOpenService.Call(hSCM, uintptr(unsafe.Pointer(svcName)), 0x0004) // SERVICE_QUERY_STATUS
	if hSvc == 0 {
		return SvcNotInstalled
	}
	defer pCloseServiceHandle.Call(hSvc)

	type SERVICE_STATUS struct {
		DwServiceType             uint32
		DwCurrentState            uint32
		DwControlsAccepted        uint32
		DwWin32ExitCode           uint32
		DwServiceSpecificExitCode uint32
		DwCheckPoint              uint32
		DwWaitHint                uint32
	}
	var status SERVICE_STATUS
	r, _, _ := pQueryServiceStatus.Call(hSvc, uintptr(unsafe.Pointer(&status)))
	if r == 0 {
		return SvcNotInstalled
	}

	if status.DwCurrentState == 4 {
		return SvcRunning
	}
	if status.DwCurrentState == 1 {
		return SvcStopped
	}
	return SvcOther
}
