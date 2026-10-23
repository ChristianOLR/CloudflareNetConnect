package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

func (o *Orchestrator) listenTCPRoute(ctx context.Context, accountID string, route config.TunnelRoute) {
	addr := fmt.Sprintf("127.0.0.1:%d", route.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[ROUTE-ERROR:%s] No se pudo escuchar en %s: %v\n", accountID, addr, err)
		return
	}
	defer listener.Close()

	log.Printf("[ROUTE-OK:%s] Enrutador local nativo activo en %s -> %s (Zero Trust WebSocket)\n", accountID, addr, route.Hostname)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		go o.handleTCPConnection(ctx, conn, route.Hostname, accountID)
	}
}

func (o *Orchestrator) handleTCPConnection(ctx context.Context, localConn net.Conn, targetHostname, accountID string) {
	defer localConn.Close()

	wsURL := url.URL{Scheme: "wss", Host: targetHostname, Path: "/"}
	headers := make(http.Header)

	o.mu.RLock()
	var token string
	for _, a := range o.cfg.Accounts {
		if a.ID == accountID || a.AccountID == accountID {
			token = a.APIToken
			break
		}
	}
	o.mu.RUnlock()

	if token != "" {
		headers.Set("Cf-Access-Token", token)
		headers.Set("Cookie", fmt.Sprintf("CF_Authorization=%s", token))
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{ServerName: targetHostname},
		HandshakeTimeout: 7 * time.Second,
	}

	wsConn, resp, err := dialer.DialContext(ctx, wsURL.String(), headers)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		log.Printf("[WS-ERR:%s] Fallo de enlace WebSocket hacia %s (HTTP %d): %v\n", accountID, targetHostname, status, err)
		return
	}
	defer wsConn.Close()

	done := make(chan struct{}, 2)

	// TCP Local -> WebSocket Remote
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, rErr := localConn.Read(buf)
			if n > 0 {
				if wErr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); wErr != nil {
					break
				}
			}
			if rErr != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	// WebSocket Remote -> TCP Local
	go func() {
		for {
			msgType, data, rErr := wsConn.ReadMessage()
			if rErr != nil {
				break
			}
			if msgType == websocket.BinaryMessage || msgType == websocket.TextMessage {
				if _, wErr := localConn.Write(data); wErr != nil {
					break
				}
			}
		}
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}
}
