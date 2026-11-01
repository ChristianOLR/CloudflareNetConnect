package wireguard

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const cfZeroTrustDefaultPeer = "162.159.193.1:2408"

type cfPeerConfig struct {
	PublicKey string `json:"public_key"`
	Endpoint  struct {
		V4    string `json:"v4"`
		Host  string `json:"host"`
		Ports []int  `json:"ports"`
	} `json:"endpoint"`
}

func extractPeerFromConfig(peers []cfPeerConfig, addrV4 string) (pubKey, endpoint string) {
	pubKey = cfPeerPubKey
	defaultPort := 2408
	if len(peers) > 0 {
		if strings.TrimSpace(peers[0].PublicKey) != "" {
			pubKey = strings.TrimSpace(peers[0].PublicKey)
		}
		if len(peers[0].Endpoint.Ports) > 0 && peers[0].Endpoint.Ports[0] > 0 {
			defaultPort = peers[0].Endpoint.Ports[0]
		}
		endpoint = normalizeEndpointAddr(peers[0].Endpoint.V4, defaultPort)
		if endpoint == "" {
			endpoint = normalizeEndpointAddr(peers[0].Endpoint.Host, defaultPort)
		}
	}
	if endpoint == "" {
		if isZeroTrustAddr(addrV4) {
			endpoint = cfZeroTrustDefaultPeer
		} else {
			endpoint = cfDefaultPeer
		}
	}
	return pubKey, endpoint
}

func normalizeEndpointAddr(raw string, defaultPort int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	host, portStr, err := net.SplitHostPort(raw)
	if err != nil {
		return fmt.Sprintf("%s:%d", raw, defaultPort)
	}
	if portStr == "" || portStr == "0" {
		return fmt.Sprintf("%s:%d", host, defaultPort)
	}
	return net.JoinHostPort(host, portStr)
}

func isZeroTrustAddr(addrV4 string) bool {
	ipStr := strings.TrimSpace(strings.Split(addrV4, "/")[0])
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return strings.HasPrefix(ipStr, "100.")
	}
	_, ztNet, err := net.ParseCIDR("100.64.0.0/10")
	return err == nil && ztNet.Contains(ip)
}

func needsZeroTrustPeerBackfill(peerEndpoint, addrV4 string) bool {
	ep := strings.TrimSpace(peerEndpoint)
	if ep == "" {
		return true
	}
	if isZeroTrustAddr(addrV4) {
		if strings.Contains(ep, "engage.cloudflareclient.com") || strings.HasPrefix(ep, "162.159.192.") {
			return true
		}
	}
	return false
}

func fetchRegisteredPeer(deviceID, token, addrV4 string) (pubKey, endpoint string) {
	if deviceID == "" || token == "" {
		return extractPeerFromConfig(nil, addrV4)
	}
	url := fmt.Sprintf("%s/%s", cfRegEndpoint, deviceID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return extractPeerFromConfig(nil, addrV4)
	}
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return extractPeerFromConfig(nil, addrV4)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return extractPeerFromConfig(nil, addrV4)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return extractPeerFromConfig(nil, addrV4)
	}
	var parsed struct {
		Config struct {
			Peers []cfPeerConfig `json:"peers"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return extractPeerFromConfig(nil, addrV4)
	}
	return extractPeerFromConfig(parsed.Config.Peers, addrV4)
}
