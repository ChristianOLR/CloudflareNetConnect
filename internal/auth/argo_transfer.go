package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/box"
)

const (
	baseLoginURL = "https://dash.cloudflare.com/argotunnel"
	baseStoreURL = "https://login.cloudflareaccess.org/"
	userAgent    = "DEV"
)

// AccessPreset define un portal Zero Trust SSO que no requiere elegir dominio ni pulsar Authorize.
type AccessPreset struct {
	Key       string
	Name      string
	AccountID string
	AppHost   string
	AppAUD    string
}

var Presets = map[string]AccessPreset{
	"olrsoft": {
		Key:       "olrsoft",
		Name:      "OLRSoft (olrcloud.com)",
		AccountID: "9940795d76034a3a83ec23ab25311965",
		AppHost:   "syncnetdb.olrcloud.com",
		AppAUD:    "8a85a99b2188bcf5e44a96007838a64a1a128c224a60a5ff82e2980f6140f838",
	},
	"hispaserver": {
		Key:       "hispaserver",
		Name:      "HispaServer (hispaserver.org)",
		AccountID: "594305aae51efdc7ea200ed8d00838ea",
		AppHost:   "logs.hispaserver.org",
		AppAUD:    "48b31002ef330a753955cd9245dba53c05308bd092ad83dfe7655c06a3e71cf7",
	},
}

type OriginCert struct {
	ZoneID    string `json:"zoneID"`
	AccountID string `json:"accountID"`
	APIToken  string `json:"apiToken"`
	Endpoint  string `json:"endpoint,omitempty"`
}

type ssoTokenResponse struct {
	AppToken string `json:"app_token"`
	OrgToken string `json:"org_token"`
}

type naclPair struct {
	pub  *[32]byte
	priv *[32]byte
}

func generateTransferKeyPair() (*naclPair, string, error) {
	pubKey, privKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	pubB64 := base64.URLEncoding.EncodeToString(pubKey[:])
	return &naclPair{pub: pubKey, priv: privKey}, pubB64, nil
}

func (kp *naclPair) decryptBox(data []byte, senderPubB64 string) ([]byte, error) {
	decodedBuf, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		decodedBuf = data
	}
	if len(decodedBuf) < 24 {
		return nil, fmt.Errorf("mensaje demasiado corto para nonce")
	}
	var nonce [24]byte
	copy(nonce[:], decodedBuf[:24])

	pubBytes, err := base64.URLEncoding.DecodeString(senderPubB64)
	if err != nil || len(pubBytes) < 32 {
		return nil, fmt.Errorf("clave pública remitente inválida")
	}
	var senderKey [32]byte
	copy(senderKey[:], pubBytes[:32])

	decrypted, ok := box.Open(nil, decodedBuf[24:], &nonce, &senderKey, kp.priv)
	if !ok {
		return nil, fmt.Errorf("fallo al desencriptar box NaCl")
	}
	return decrypted, nil
}

func buildArgoLoginURL(pubKey string) string {
	u, _ := url.Parse(baseLoginURL)
	q := u.Query()
	q.Set("callback", baseStoreURL+pubKey)
	q.Set("aud", "")
	u.RawQuery = q.Encode()
	return u.String()
}

// buildAccessSSOURL construye la URL oficial Zero Trust CLI que autentica en 0 clics y cierra la pestaña sola.
func buildAccessSSOURL(appHost, appAUD, pubKey string) string {
	baseURL := &url.URL{Scheme: "https", Host: appHost}
	q := baseURL.Query()
	q.Set("token", pubKey)
	q.Set("aud", appAUD)
	baseURL.RawQuery = q.Encode()

	redirectStr := baseURL.String()
	q.Set("redirect_url", redirectStr)
	q.Set("send_org_token", "true")
	q.Set("edge_token_transfer", "true")
	q.Set("close_interstitial", "true")
	baseURL.RawQuery = q.Encode()
	baseURL.Path = "cdn-cgi/access/cli"
	return baseURL.String()
}

func extractEmailFromJWT(jwtStr string) string {
	parts := strings.Split(jwtStr, ".")
	if len(parts) < 2 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(raw, &claims)
	return claims.Email
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
				if raw, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(block.Bytes))); decErr == nil {
					_ = json.Unmarshal(raw, &cert)
				}
			}
			if cert.APIToken == "" {
				cert.APIToken = strings.TrimSpace(string(block.Bytes))
			}
		}
		block, rest = pem.Decode(rest)
	}
	if cert.APIToken == "" {
		_ = json.Unmarshal(blocks, &cert)
	}
	if cert.APIToken == "" {
		return nil, fmt.Errorf("no se encontró apiToken (%d bytes)", len(blocks))
	}
	return &cert, nil
}

func (cs *CallbackServer) pollCloudflareTransfer(kp *naclPair, pubKey string, preset *AccessPreset) {
	client := &http.Client{Timeout: 65 * time.Second}
	pollURL := baseStoreURL + pubKey
	if preset != nil {
		pollURL = baseStoreURL + "transfer/" + pubKey
	}
	log.Printf("[ARGO-POLL] Escuchando transferencia en: %s\n", pollURL)

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
			case <-time.After(1500 * time.Millisecond):
				continue
			}
		}

		body, _ := io.ReadAll(resp.Body)
		svcPubKey := resp.Header.Get("service-public-key")
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK && len(body) > 0 {
			payload := body
			if svcPubKey != "" && kp != nil {
				if dec, decErr := kp.decryptBox(body, svcPubKey); decErr == nil {
					payload = dec
				}
			}
			if preset != nil {
				var ssoResp ssoTokenResponse
				if json.Unmarshal(payload, &ssoResp) == nil && ssoResp.AppToken != "" {
					email := extractEmailFromJWT(ssoResp.AppToken)
					log.Printf("[ARGO-POLL] ✅ SSO Zero Trust completado para %s (%s)\n", preset.Name, email)
					cs.emitResult(AuthResult{
						Token:       ssoResp.AppToken,
						AccountID:   preset.AccountID,
						AccountName: preset.Name,
						UserEmail:   email,
					})
					return
				}
			}
			if cert, decErr := decodeOriginCert(payload); decErr == nil && cert.APIToken != "" {
				cs.emitResult(AuthResult{Token: cert.APIToken, AccountID: cert.AccountID, ZoneID: cert.ZoneID})
				return
			}
		}

		select {
		case <-cs.stopCh:
			return
		case <-time.After(1200 * time.Millisecond):
		}
	}
}
