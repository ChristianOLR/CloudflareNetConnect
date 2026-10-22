package tunnel

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/olrsoft/cloudflarenetconnect/internal/config"
)

func (o *Orchestrator) listenTCPRoute(ctx context.Context, accountID string, route config.TunnelRoute) {
	addr := fmt.Sprintf("127.0.0.1:%d", route.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[ROUTE-ERROR:%s] No se pudo escuchar en %s: %v\n", accountID, addr, err)
		return
	}
	defer listener.Close()

	log.Printf("[ROUTE-OK:%s] Enrutador local activo en %s -> %s\n", accountID, addr, route.Hostname)

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
		go o.handleTCPConnection(ctx, conn, route.Hostname)
	}
}

func (o *Orchestrator) handleTCPConnection(ctx context.Context, localConn net.Conn, targetHostname string) {
	defer localConn.Close()

	remoteConn, err := net.DialTimeout("tcp", targetHostname+":443", 5*time.Second)
	if err != nil {
		log.Printf("[TCP-ERR] Fallo de conexión hacia %s: %v\n", targetHostname, err)
		return
	}
	defer remoteConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}
}
