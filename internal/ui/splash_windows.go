//go:build windows

package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type SplashWindow struct {
	splashURL string
	cmd       *exec.Cmd
	mu        sync.Mutex
	done      chan struct{}
}

func NewSplashWindow(splashURL string) *SplashWindow {
	return &SplashWindow{
		splashURL: splashURL,
		done:      make(chan struct{}),
	}
}

func (s *SplashWindow) Show() {
	go s.launchAppWindow()
}

func (s *SplashWindow) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *SplashWindow) Wait() {
	<-s.done
}

func (s *SplashWindow) launchAppWindow() {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
	}

	var binPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			binPath = c
			break
		}
	}

	userDataDir := filepath.Join(os.TempDir(), "CloudflareNetConnect_SplashUI")
	_ = os.MkdirAll(userDataDir, 0700)

	s.mu.Lock()
	if binPath != "" {
		s.cmd = exec.Command(
			binPath,
			"--app="+s.splashURL,
			"--window-size=410,490",
			"--window-position=460,220",
			"--user-data-dir="+userDataDir,
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-extensions",
		)
	} else {
		s.cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", s.splashURL)
	}
	_ = s.cmd.Start()
	s.mu.Unlock()

	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	s.Close()
}

// CopyToClipboard copia texto al portapapeles nativo de Windows.
func CopyToClipboard(text string) {
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	pOpenClipboard := u32.NewProc("OpenClipboard")
	pEmptyClipboard := u32.NewProc("EmptyClipboard")
	pSetClipboardData := u32.NewProc("SetClipboardData")
	pCloseClipboard := u32.NewProc("CloseClipboard")
	pGlobalAlloc := k32.NewProc("GlobalAlloc")
	pGlobalLock := k32.NewProc("GlobalLock")
	pGlobalUnlock := k32.NewProc("GlobalUnlock")

	r, _, _ := pOpenClipboard.Call(0)
	if r == 0 {
		return
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()

	utf16, _ := windows.UTF16FromString(text)
	bytesCount := len(utf16) * 2

	hMem, _, _ := pGlobalAlloc.Call(0x0042, uintptr(bytesCount))
	if hMem == 0 {
		return
	}
	ptr, _, _ := pGlobalLock.Call(hMem)
	if ptr != 0 {
		dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
		copy(dst, utf16)
		pGlobalUnlock.Call(hMem)
		pSetClipboardData.Call(13, hMem) // CF_UNICODETEXT
	}
}

// OpenBrowserURL abre una URL en el navegador predeterminado de Windows solo bajo petición explícita.
func OpenBrowserURL(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
