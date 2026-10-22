//go:build !windows

package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	userDataDir := filepath.Join(os.TempDir(), "CloudflareNetConnect_SplashUI")
	_ = os.MkdirAll(userDataDir, 0700)

	s.mu.Lock()
	if runtime.GOOS == "darwin" {
		chromeMac := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		edgeMac := "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
		braveMac := "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
		var bin string
		for _, c := range []string{chromeMac, edgeMac, braveMac} {
			if _, err := os.Stat(c); err == nil {
				bin = c
				break
			}
		}
		if bin != "" {
			s.cmd = exec.Command(bin, "--app="+s.splashURL, "--window-size=410,490", "--user-data-dir="+userDataDir, "--no-first-run")
		} else {
			s.cmd = exec.Command("open", s.splashURL)
		}
	} else {
		var bin string
		for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				bin = p
				break
			}
		}
		if bin != "" {
			s.cmd = exec.Command(bin, "--app="+s.splashURL, "--window-size=410,490", "--user-data-dir="+userDataDir, "--no-first-run")
		} else {
			s.cmd = exec.Command("xdg-open", s.splashURL)
		}
	}
	_ = s.cmd.Start()
	s.mu.Unlock()

	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	s.Close()
}

// CopyToClipboard copia texto al portapapeles en macOS (pbcopy) o Linux (wl-copy / xclip).
func CopyToClipboard(text string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("pbcopy")
	} else {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

// OpenBrowserURL abre una URL en el navegador predeterminado en macOS o Linux.
func OpenBrowserURL(url string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", url).Start()
	} else {
		_ = exec.Command("xdg-open", url).Start()
	}
}
