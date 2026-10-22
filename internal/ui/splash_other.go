//go:build !windows

package ui

import (
	"log"
)

type SplashWindow struct {
	authURL string
	done    chan struct{}
}

func NewSplashWindow(authURL string) *SplashWindow {
	return &SplashWindow{
		authURL: authURL,
		done:    make(chan struct{}),
	}
}

func (s *SplashWindow) Show() {
	log.Printf("[SPLASH] Por favor visita la siguiente URL para autorizar tu cuenta:\n%s\n", s.authURL)
}

func (s *SplashWindow) Close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *SplashWindow) Wait() {
	<-s.done
}
