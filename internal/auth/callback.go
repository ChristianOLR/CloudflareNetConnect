package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// AuthResult contiene el resultado de la autenticación Argo o manual.
type AuthResult struct {
	Token     string
	AccountID string
	ZoneID    string
	Error     error
}

// CallbackServer gestiona el polling de Cloudflare Argo y sirve la UI del Splash.
type CallbackServer struct {
	listener   net.Listener
	server     *http.Server
	port       int
	authURL    string
	splashHTML string
	onCopy     func(string)
	onOpenURL  func(string)
	resultCh   chan AuthResult
	stopCh     chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	once       sync.Once
	mu         sync.RWMutex
	done       bool
	accName    string
	errMsg     string
}

// NewCallbackServer inicia el servidor local efímero y el polling hacia login.cloudflareaccess.org.
func NewCallbackServer(splashHTMLFn func(authURL string) string, onCopy, onOpenURL func(string)) (*CallbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("error al abrir puerto local: %w", err)
	}

	pubKey, err := generateTransferKey()
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("error generando clave de transferencia: %w", err)
	}

	authURL := buildArgoLoginURL(pubKey)
	ctx, cancel := context.WithCancel(context.Background())

	port := listener.Addr().(*net.TCPAddr).Port
	cs := &CallbackServer{
		listener:   listener,
		port:       port,
		authURL:    authURL,
		splashHTML: splashHTMLFn(authURL),
		onCopy:     onCopy,
		onOpenURL:  onOpenURL,
		resultCh:   make(chan AuthResult, 1),
		stopCh:     make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/splash", cs.handleSplash)
	mux.HandleFunc("/status", cs.handleStatus)
	mux.HandleFunc("/copy", cs.handleCopy)
	mux.HandleFunc("/open-browser", cs.handleOpenBrowser)
	mux.HandleFunc("/cancel", cs.handleCancel)
	mux.HandleFunc("/manual-token", cs.handleManualToken)

	cs.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() { _ = cs.server.Serve(listener) }()
	go cs.pollCloudflareTransfer(pubKey)

	return cs, nil
}

func (cs *CallbackServer) GetSplashURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/splash", cs.port)
}

func (cs *CallbackServer) GetAuthURL() string {
	return cs.authURL
}

func (cs *CallbackServer) SetAccountLinked(name string) {
	cs.mu.Lock()
	cs.accName = name
	cs.done = true
	cs.mu.Unlock()
}

func (cs *CallbackServer) WaitForToken(timeout time.Duration) (AuthResult, error) {
	select {
	case res := <-cs.resultCh:
		return res, res.Error
	case <-time.After(timeout):
		cs.Close()
		return AuthResult{}, fmt.Errorf("tiempo de espera agotado (timeout)")
	}
}

func (cs *CallbackServer) handleSplash(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprint(w, cs.splashHTML)
}

func (cs *CallbackServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	cs.mu.RLock()
	resp := map[string]any{
		"done":    cs.done,
		"account": cs.accName,
		"error":   cs.errMsg,
	}
	cs.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (cs *CallbackServer) handleCopy(w http.ResponseWriter, r *http.Request) {
	if cs.onCopy != nil {
		cs.onCopy(cs.authURL)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (cs *CallbackServer) handleOpenBrowser(w http.ResponseWriter, r *http.Request) {
	if cs.onOpenURL != nil {
		cs.onOpenURL(cs.authURL)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (cs *CallbackServer) handleCancel(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	cs.emitResult(AuthResult{Error: fmt.Errorf("cancelado por el usuario")})
}

func (cs *CallbackServer) handleManualToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Token != "" {
		cs.emitResult(AuthResult{Token: body.Token})
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
}

func (cs *CallbackServer) emitResult(res AuthResult) {
	cs.once.Do(func() {
		close(cs.stopCh)
		cs.cancel()
		cs.resultCh <- res
	})
}

func (cs *CallbackServer) Close() {
	cs.emitResult(AuthResult{Error: fmt.Errorf("cerrado")})
	if cs.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = cs.server.Shutdown(ctx)
	}
}
