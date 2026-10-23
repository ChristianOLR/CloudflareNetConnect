//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void showMacSplash(const char* authURL, const void* pngBytes, int pngLen);
void updateMacSplashSuccess(const char* accName);
void closeMacSplash(void);
*/
import "C"

import (
	"os/exec"
	"strings"
	"sync"
	"unsafe"

	"github.com/chgarcia79/cloudflarenetconnect/assets"
)

var (
	g_activeSplash *SplashWindow
	g_splashMu     sync.Mutex
)

type SplashWindow struct {
	authURL   string
	done      chan struct{}
	closeOnce sync.Once
}

func NewSplashWindow(authURL string) *SplashWindow {
	return &SplashWindow{
		authURL: authURL,
		done:    make(chan struct{}),
	}
}

func (s *SplashWindow) Show() {
	g_splashMu.Lock()
	g_activeSplash = s
	g_splashMu.Unlock()

	CopyToClipboard(s.authURL)

	cURL := C.CString(s.authURL)
	defer C.free(unsafe.Pointer(cURL))
	png := assets.CloudflarePNG
	C.showMacSplash(cURL, unsafe.Pointer(&png[0]), C.int(len(png)))
}

func (s *SplashWindow) SetSuccess(accName string) {
	cName := C.CString(accName)
	defer C.free(unsafe.Pointer(cName))
	C.updateMacSplashSuccess(cName)
}

func (s *SplashWindow) Close() {
	C.closeMacSplash()
	s.closeOnce.Do(func() { close(s.done) })
}

func (s *SplashWindow) Wait() {
	<-s.done
}

//export goOnMacSplashClosed
func goOnMacSplashClosed() {
	g_splashMu.Lock()
	sp := g_activeSplash
	g_activeSplash = nil
	g_splashMu.Unlock()
	if sp != nil {
		sp.closeOnce.Do(func() { close(sp.done) })
	}
}

//export goOnMacSplashCopy
func goOnMacSplashCopy(cURL *C.char) {
	if cURL != nil {
		CopyToClipboard(C.GoString(cURL))
	}
}

func CopyToClipboard(text string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}
