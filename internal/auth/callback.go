package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AuthResult contiene el resultado de la autenticación Argo o SSO Zero Trust.
type AuthResult struct {
	Token       string
	AccountID   string
	AccountName string
	UserEmail   string
	ZoneID      string
	Error       error
}

// CallbackServer gestiona el polling oficial hacia login.cloudflareaccess.org sin abrir puertos locales.
type CallbackServer struct {
	authURL  string
	resultCh chan AuthResult
	stopCh   chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	once     sync.Once
}

// NewCallbackServer genera la clave efímera Curve25519 y lanza el long-polling hacia Cloudflare (SSO o Argo).
func NewCallbackServer(presetKey string) (*CallbackServer, error) {
	kp, pubKey, err := generateTransferKeyPair()
	if err != nil {
		return nil, fmt.Errorf("error generando clave de transferencia: %w", err)
	}

	var preset *AccessPreset
	authURL := buildArgoLoginURL(pubKey)
	if p, ok := Presets[presetKey]; ok {
		preset = &p
		authURL = buildAccessSSOURL(p.AppHost, p.AppAUD, pubKey)
	}

	ctx, cancel := context.WithCancel(context.Background())

	cs := &CallbackServer{
		authURL:  authURL,
		resultCh: make(chan AuthResult, 1),
		stopCh:   make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}

	go cs.pollCloudflareTransfer(kp, pubKey, preset)
	return cs, nil
}

// GetAuthURL devuelve el enlace oficial de identificación de Cloudflare.
func (cs *CallbackServer) GetAuthURL() string {
	return cs.authURL
}

// WaitForToken espera a que el usuario autorice la cuenta o transcurra el timeout.
func (cs *CallbackServer) WaitForToken(timeout time.Duration) (AuthResult, error) {
	select {
	case res := <-cs.resultCh:
		return res, res.Error
	case <-time.After(timeout):
		cs.Close()
		return AuthResult{}, fmt.Errorf("tiempo de espera agotado (timeout)")
	}
}

func (cs *CallbackServer) emitResult(res AuthResult) {
	cs.once.Do(func() {
		close(cs.stopCh)
		cs.cancel()
		cs.resultCh <- res
	})
}

// Close detiene el polling de fondo inmediatamente.
func (cs *CallbackServer) Close() {
	cs.emitResult(AuthResult{Error: fmt.Errorf("cancelado")})
}
