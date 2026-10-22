package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseLoginURL = "https://dash.cloudflare.com/argotunnel"
	baseStoreURL = "https://login.cloudflareaccess.org/"
	userAgent    = "cloudflared/2026.2.0"
)

// OriginCert representa el payload decodificado del bloque ARGO TUNNEL TOKEN.
type OriginCert struct {
	ZoneID    string `json:"zoneID"`
	AccountID string `json:"accountID"`
	APIToken  string `json:"apiToken"`
	Endpoint  string `json:"endpoint,omitempty"`
}

func generateTransferKey() (string, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(key[:]), nil
}

func buildArgoLoginURL(pubKey string) string {
	u, _ := url.Parse(baseLoginURL)
	q := u.Query()
	q.Set("callback", baseStoreURL+pubKey)
	q.Set("aud", "")
	u.RawQuery = q.Encode()
	return u.String()
}

func decodeOriginCert(blocks []byte) (*OriginCert, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("certificado vacío")
	}
	var cert OriginCert
	block, rest := pem.Decode(blocks)
	for block != nil {
		if block.Type == "ARGO TUNNEL TOKEN" {
			if err := json.Unmarshal(block.Bytes, &cert); err != nil {
				// Fallback si viniese doblemente codificado en base64
				if raw, decErr := base64.StdEncoding.DecodeString(string(block.Bytes)); decErr == nil {
					_ = json.Unmarshal(raw, &cert)
				}
			}
		}
		block, rest = pem.Decode(rest)
	}
	if cert.APIToken == "" {
		return nil, fmt.Errorf("no se encontró apiToken en el certificado recibido")
	}
	return &cert, nil
}

func (cs *CallbackServer) pollCloudflareTransfer(pubKey string) {
	client := &http.Client{Timeout: 65 * time.Second}
	pollURL := baseStoreURL + pubKey

	for {
		select {
		case <-cs.stopCh:
			return
		default:
		}

		req, err := http.NewRequestWithContext(cs.ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			select {
			case <-cs.stopCh:
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK && len(body) > 0 {
			cert, decErr := decodeOriginCert(body)
			if decErr == nil && cert.APIToken != "" {
				cs.emitResult(AuthResult{
					Token:     cert.APIToken,
					AccountID: cert.AccountID,
					ZoneID:    cert.ZoneID,
				})
				return
			}
		}

		select {
		case <-cs.stopCh:
			return
		case <-time.After(1500 * time.Millisecond):
		}
	}
}
