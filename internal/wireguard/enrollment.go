package wireguard

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/curve25519"
	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

const (
	cfRegEndpoint = "https://api.cloudflareclient.com/v0a2158/reg"
	cfPeerPubKey  = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="
	cfDefaultPeer = "engage.cloudflareclient.com:2408"
)

type EnrollmentResult struct {
	DeviceID    string
	PrivateKey  string
	PublicKey   string
	AddressIPv4 string
	AddressIPv6 string
	PeerPubKey  string
	PeerEndpoint string
}

// GenerateKeyPair genera un par de claves Curve25519 para WireGuard en Base64.
func GenerateKeyPair() (string, string, error) {
	var privKey [32]byte
	if _, err := rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("error generando clave privada: %w", err)
	}
	privKey[0] &= 248
	privKey[31] = (privKey[31] & 127) | 64

	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	privB64 := base64.StdEncoding.EncodeToString(privKey[:])
	pubB64 := base64.StdEncoding.EncodeToString(pubKey[:])
	return privB64, pubB64, nil
}

// EnsureEnrolled verifica si la cuenta ya tiene perfil WireGuard o la registra automáticamente.
func EnsureEnrolled(pn *config.PrivateNetworkConfig) (*EnrollmentResult, error) {
	if pn == nil {
		return nil, fmt.Errorf("configuración de red privada nula")
	}
	if pn.DeviceID != "" && pn.PrivateKey != "" && pn.AddressIPv4 != "" {
		return &EnrollmentResult{
			DeviceID:     pn.DeviceID,
			PrivateKey:   pn.PrivateKey,
			AddressIPv4:  pn.AddressIPv4,
			AddressIPv6:  pn.AddressIPv6,
			PeerPubKey:   cfPeerPubKey,
			PeerEndpoint: cfDefaultPeer,
		}, nil
	}

	privKey, pubKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"key":           pubKey,
		"install_id":    "",
		"fcm_token":     "",
		"tos":           time.Now().UTC().Format(time.RFC3339Nano),
		"model":         "PC",
		"serial_number": "",
		"locale":        "es_ES",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, cfRegEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "okhttp/3.12.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo conectando al endpoint de enrolamiento: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("error enrolando dispositivo (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		ID     string `json:"id"`
		Config struct {
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"addresses"`
			} `json:"interface"`
		} `json:"config"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("error parseando respuesta de enrolamiento: %w", err)
	}

	res := &EnrollmentResult{
		DeviceID:     parsed.ID,
		PrivateKey:   privKey,
		PublicKey:    pubKey,
		AddressIPv4:  parsed.Config.Interface.Addresses.V4,
		AddressIPv6:  parsed.Config.Interface.Addresses.V6,
		PeerPubKey:   cfPeerPubKey,
		PeerEndpoint: cfDefaultPeer,
	}

	pn.DeviceID = res.DeviceID
	pn.PrivateKey = res.PrivateKey
	pn.AddressIPv4 = res.AddressIPv4
	pn.AddressIPv6 = res.AddressIPv6
	return res, nil
}
