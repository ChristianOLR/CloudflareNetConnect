//go:build windows

package ui

import (
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/assets"
	"golang.org/x/sys/windows"
)

type paintStruct struct {
	hdc         windows.Handle
	fErase      int32
	rcPaint     struct{ left, top, right, bottom int32 }
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type rect struct {
	left, top, right, bottom int32
}

func (s *SplashWindow) paintCard(hWnd windows.Handle) {
	u32 := windows.NewLazySystemDLL("user32.dll")
	gdi := windows.NewLazySystemDLL("gdi32.dll")

	pBeginPaint := u32.NewProc("BeginPaint")
	pEndPaint := u32.NewProc("EndPaint")
	pCreateCompatibleDC := gdi.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap := gdi.NewProc("CreateCompatibleBitmap")
	pSelectObject := gdi.NewProc("SelectObject")
	pDeleteObject := gdi.NewProc("DeleteObject")
	pDeleteDC := gdi.NewProc("DeleteDC")
	pBitBlt := gdi.NewProc("BitBlt")
	pCreateSolidBrush := gdi.NewProc("CreateSolidBrush")
	pCreatePen := gdi.NewProc("CreatePen")
	pRoundRect := gdi.NewProc("RoundRect")
	pEllipse := gdi.NewProc("Ellipse")
	pSetBkMode := gdi.NewProc("SetBkMode")
	pSetTextColor := gdi.NewProc("SetTextColor")
	pCreateFontW := gdi.NewProc("CreateFontW")
	pDrawTextW := u32.NewProc("DrawTextW")
	pDrawIconEx := u32.NewProc("DrawIconEx")

	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(uintptr(hWnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hWnd), uintptr(unsafe.Pointer(&ps)))

	w, h := int32(380), int32(410)
	memDC, _, _ := pCreateCompatibleDC.Call(hdc)
	defer pDeleteDC.Call(memDC)
	memBM, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	oldBM, _, _ := pSelectObject.Call(memDC, memBM)
	defer func() {
		pSelectObject.Call(memDC, oldBM)
		pDeleteObject.Call(memBM)
	}()

	// 1. Fondo de tarjeta redondeada (#0B0F19 con borde #334155)
	bgBrush, _, _ := pCreateSolidBrush.Call(0x00190F0B) // BGR(11, 15, 25)
	borderPen, _, _ := pCreatePen.Call(0, 2, 0x00554133)
	oB, _, _ := pSelectObject.Call(memDC, bgBrush)
	oP, _, _ := pSelectObject.Call(memDC, borderPen)
	pRoundRect.Call(memDC, 1, 1, uintptr(w-1), uintptr(h-1), 24, 24)
	pSelectObject.Call(memDC, oB)
	pSelectObject.Call(memDC, oP)
	pDeleteObject.Call(bgBrush)
	pDeleteObject.Call(borderPen)

	// 2. Anillos concéntricos animados de Cloudflare
	cx, cy := int32(w/2), int32(76)
	s.mu.Lock()
	tick := s.animTick
	copied := s.copiedState
	statusTxt := s.statusText
	isSuccess := s.successState
	s.mu.Unlock()

	ringColors := []uintptr{0x002081F4, 0x003FADFA, 0x001A559E}
	for i := 0; i < 3; i++ {
		phase := (tick + i*12) % 36
		radius := int32(28 + (phase * 38 / 36))
		col := ringColors[i]
		if phase > 26 {
			col = 0x002B221C
		}
		pen, _, _ := pCreatePen.Call(0, 2, col)
		nullBrush, _, _ := gdi.NewProc("GetStockObject").Call(5)
		op1, _, _ := pSelectObject.Call(memDC, pen)
		ob1, _, _ := pSelectObject.Call(memDC, nullBrush)
		pEllipse.Call(memDC, uintptr(cx-radius), uintptr(cy-radius), uintptr(cx+radius), uintptr(cy+radius))
		pSelectObject.Call(memDC, op1)
		pSelectObject.Call(memDC, ob1)
		pDeleteObject.Call(pen)
	}

	coreBrush, _, _ := pCreateSolidBrush.Call(0x00291E14)
	corePen, _, _ := pCreatePen.Call(0, 1, 0x002081F4)
	ob2, _, _ := pSelectObject.Call(memDC, coreBrush)
	op2, _, _ := pSelectObject.Call(memDC, corePen)
	pEllipse.Call(memDC, uintptr(cx-34), uintptr(cy-34), uintptr(cx+34), uintptr(cy+34))
	pSelectObject.Call(memDC, ob2)
	pSelectObject.Call(memDC, op2)
	pDeleteObject.Call(coreBrush)
	pDeleteObject.Call(corePen)

	if s.hLogoIcon == 0 {
		pCreateIconFromResourceEx := u32.NewProc("CreateIconFromResourceEx")
		png := assets.CloudflarePNG
		hIc, _, _ := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, 0x00030000, 56, 56, 0)
		s.hLogoIcon = windows.Handle(hIc)
	}
	if s.hLogoIcon != 0 {
		pDrawIconEx.Call(memDC, uintptr(cx-28), uintptr(cy-28), uintptr(s.hLogoIcon), 56, 56, 0, 0, 3)
	}

	// 3. Textos tipográficos Segoe UI
	pSetBkMode.Call(memDC, 1)
	fontFace, _ := windows.UTF16PtrFromString("Segoe UI")
	fTitle, _, _ := pCreateFontW.Call(21, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(fontFace)))
	fHint, _, _ := pCreateFontW.Call(13, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(fontFace)))
	fSub, _, _ := pCreateFontW.Call(15, 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(fontFace)))
	fBtn, _, _ := pCreateFontW.Call(16, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(fontFace)))
	defer func() {
		pDeleteObject.Call(fTitle)
		pDeleteObject.Call(fHint)
		pDeleteObject.Call(fSub)
		pDeleteObject.Call(fBtn)
	}()

	drawCentered := func(font uintptr, color uintptr, top, bottom int32, text string) {
		pSelectObject.Call(memDC, font)
		pSetTextColor.Call(memDC, color)
		rc := rect{16, top, w - 16, bottom}
		u16, _ := windows.UTF16PtrFromString(text)
		pDrawTextW.Call(memDC, uintptr(unsafe.Pointer(u16)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), 0x01|0x04|0x20)
	}

	drawCentered(fTitle, 0x00FFFFFF, 138, 162, "Vincular Cuenta Cloudflare")
	drawCentered(fHint, 0x00C8B8A8, 164, 182, "1. Abre o pega el enlace en tu navegador")
	drawCentered(fHint, 0x00C8B8A8, 180, 198, "2. Selecciona el dominio y pulsa «Authorize»")

	statusColor := uintptr(0x0074BAFD)
	if isSuccess {
		statusColor = 0x0099D334
	}
	drawCentered(fSub, statusColor, 198, 220, statusTxt)

	// 4. Botón 1: Copiar Enlace [32..348, 224..268]
	btnBg := uintptr(0x002081F4) // Naranja Cloudflare BGR(244, 129, 32)
	btnTxt := "📋  Copiar Enlace de Identificación"
	if copied {
		btnBg = 0x0081B910
		btnTxt = "✅  ¡Enlace Copiado! Pégalo en tu navegador"
	}
	bBrush, _, _ := pCreateSolidBrush.Call(btnBg)
	bPen, _, _ := pCreatePen.Call(0, 1, btnBg)
	pSelectObject.Call(memDC, bBrush)
	pSelectObject.Call(memDC, bPen)
	pRoundRect.Call(memDC, 32, 224, uintptr(w-32), 268, 14, 14)
	pDeleteObject.Call(bBrush)
	pDeleteObject.Call(bPen)
	drawCentered(fBtn, 0x00190F0B, 224, 268, btnTxt)

	// 5. Botón 2: Abrir en Navegador [32..348, 280..322]
	navBrush, _, _ := pCreateSolidBrush.Call(0x00473322) // Azul pizarra oscuro
	navPen, _, _ := pCreatePen.Call(0, 1, 0x007A5A3D)
	pSelectObject.Call(memDC, navBrush)
	pSelectObject.Call(memDC, navPen)
	pRoundRect.Call(memDC, 32, 280, uintptr(w-32), 322, 14, 14)
	pDeleteObject.Call(navBrush)
	pDeleteObject.Call(navPen)
	drawCentered(fBtn, 0x00F8FAFC, 280, 322, "🌐  Abrir en Navegador")

	// 6. Botón 3: Cancelar [125..255, 340..374]
	cBrush, _, _ := pCreateSolidBrush.Call(0x00291E16)
	cPen, _, _ := pCreatePen.Call(0, 1, 0x00554133)
	pSelectObject.Call(memDC, cBrush)
	pSelectObject.Call(memDC, cPen)
	pRoundRect.Call(memDC, 125, 340, uintptr(w-125), 374, 12, 12)
	pDeleteObject.Call(cBrush)
	pDeleteObject.Call(cPen)
	drawCentered(fSub, 0x00B8A394, 340, 374, "Cancelar")

	pBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, 0x00CC0020)
}
