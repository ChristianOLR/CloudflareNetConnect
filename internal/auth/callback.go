package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// AuthResult contiene el resultado de la autenticación.
type AuthResult struct {
	Token string
	Error error
}

// CallbackServer gestiona la captura efímera del token OAuth/API.
type CallbackServer struct {
	listener net.Listener
	server   *http.Server
	port     int
	resultCh chan AuthResult
	once     sync.Once
}

// NewCallbackServer inicia un listener en un puerto local efímero.
func NewCallbackServer() (*CallbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("error al abrir puerto local para callback: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	cs := &CallbackServer{
		listener: listener,
		port:     port,
		resultCh: make(chan AuthResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cs.handleCallback)

	cs.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		_ = cs.server.Serve(listener)
	}()

	return cs, nil
}

// GetPort devuelve el puerto asignado.
func (cs *CallbackServer) GetPort() int {
	return cs.port
}

// GetAuthURL genera la URL para autenticación con el redirect_uri configurado.
func (cs *CallbackServer) GetAuthURL() string {
	return fmt.Sprintf("https://dash.cloudflare.com/profile/api-tokens?redirect_uri=http://localhost:%d/callback", cs.port)
}

// WaitForToken espera hasta recibir el token o que transcurra el timeout (5 min).
func (cs *CallbackServer) WaitForToken(timeout time.Duration) (string, error) {
	defer cs.Close()

	select {
	case res := <-cs.resultCh:
		return res.Token, res.Error
	case <-time.After(timeout):
		return "", fmt.Errorf("tiempo de espera agotado (timeout) esperando autenticación")
	}
}

func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.URL.Query().Get("code")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if token != "" {
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<head><title>CloudflareNetConnect</title><style>body{font-family:sans-serif;background:#0f172a;color:#fff;display:flex;justify-content:center;align-items:center;height:100vh;margin:0} .card{background:#1e293b;padding:2rem;border-radius:12px;text-align:center;box-shadow:0 10px 25px rgba(0,0,0,0.5)} h1{color:#f38020;margin-bottom:0.5rem}</style></head>
			<body>
				<div class="card">
					<h1>☁️ CloudflareNetConnect</h1>
					<p>¡Cuenta identificada con éxito!</p>
					<small style="color:#94a3b8">Puedes cerrar esta ventana y regresar a tu escritorio.</small>
				</div>
			</body>
			</html>
		`)
		cs.emitResult(token, nil)
	} else {
		// Formulario de inserción rápida si el proveedor no inyectó token directo por querystring
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<head><title>CloudflareNetConnect - Vincular</title><style>body{font-family:sans-serif;background:#0f172a;color:#fff;display:flex;justify-content:center;align-items:center;height:100vh;margin:0} .card{background:#1e293b;padding:2rem;border-radius:12px;text-align:center;width:340px} h1{color:#f38020} input{width:90%;padding:10px;margin:15px 0;border-radius:6px;border:1px solid #334155;background:#0f172a;color:#fff} button{background:#f38020;color:#fff;border:none;padding:10px 20px;border-radius:6px;cursor:pointer;font-weight:bold}</style></head>
			<body>
				<div class="card">
					<h1>☁️ Vincular Token</h1>
					<p style="font-size:0.9rem;color:#94a3b8">Introduce tu API Token de Cloudflare:</p>
					<form method="GET" action="/callback">
						<input type="password" name="token" placeholder="Bearer Token" required />
						<button type="submit">Vincular Cuenta</button>
					</form>
				</div>
			</body>
			</html>
		`)
	}
}

func (cs *CallbackServer) emitResult(token string, err error) {
	cs.once.Do(func() {
		cs.resultCh <- AuthResult{Token: token, Error: err}
	})
}

// Close apaga el servidor efímero.
func (cs *CallbackServer) Close() {
	if cs.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = cs.server.Shutdown(ctx)
	}
}
