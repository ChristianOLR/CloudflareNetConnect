//go:build !windows

package ui

import (
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
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
	CopyToClipboard(s.authURL)
	log.Printf("[SPLASH] Enlace de identificación copiado al portapapeles: %s\n", s.authURL)
	go func() {
		if runtime.GOOS == "darwin" {
			script := `display dialog "Esperando identificación en Cloudflare...\n\nEl enlace de identificación ya se ha copiado a tu portapapeles. Pégalo en tu navegador para vincular la cuenta." with title "CloudflareNetConnect" buttons {"Copiar Enlace", "Cancelar"} default button "Copiar Enlace"`
			out, _ := exec.Command("osascript", "-e", script).CombinedOutput()
			if strings.Contains(string(out), "Copiar Enlace") {
				CopyToClipboard(s.authURL)
			}
		} else {
			_ = exec.Command("zenity", "--info", "--title=CloudflareNetConnect", "--width=360", "--text=Esperando identificación en Cloudflare...\n\nEl enlace ya está copiado en tu portapapeles. Pégalo en tu navegador.").Run()
		}
	}()
}

func (s *SplashWindow) SetSuccess(accName string) {
	log.Printf("[SPLASH] ✅ Cuenta %s vinculada con éxito.\n", accName)
}

func (s *SplashWindow) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

func (s *SplashWindow) Wait() {
	<-s.done
}

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
