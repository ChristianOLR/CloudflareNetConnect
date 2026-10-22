//go:build windows

package tray

import (
	"golang.org/x/sys/windows"
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             windows.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutOrVer    uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}
