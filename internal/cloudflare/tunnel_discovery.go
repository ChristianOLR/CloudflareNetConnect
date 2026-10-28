package cloudflare

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/chgarcia79/cloudflarenetconnect/internal/config"
)

type tunnelConfigResponse struct {
	Config struct {
		Ingress []struct {
			Hostname string `json:"hostname"`
			Service  string `json:"service"`
		} `json:"ingress"`
	} `json:"config"`
}

type dnsRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

// DiscoverRoutes descubre dinámicamente los túneles y rutas TCP configurados en Cloudflare.
func (c *Client) DiscoverRoutes(accountID, zoneID string) []config.TunnelRoute {
	var routes []config.TunnelRoute
	seen := make(map[string]bool)
	usedPorts := make(map[int]bool)

	addRoute := func(hostname string, port int, proto string) {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname == "" || port <= 0 || seen[hostname] {
			return
		}
		seen[hostname] = true
		for usedPorts[port] {
			port++
		}
		usedPorts[port] = true
		routes = append(routes, config.TunnelRoute{
			Hostname:  hostname,
			LocalPort: port,
			Protocol:  proto,
			Enabled:   true,
		})
		log.Printf("[DISCOVERY] ✅ Ruta detectada: %s -> 127.0.0.1:%d (%s)\n", hostname, port, proto)
	}

	// 1. Inspeccionar configuraciones de Ingress en Cloudflare Tunnels (cfd_tunnel)
	if accountID != "" {
		if tunnels, err := c.ListTunnels(accountID); err == nil {
			for _, t := range tunnels {
				cfgURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", accountID, t.ID)
				body, err := c.doGet(cfgURL)
				if err != nil {
					continue
				}
				var parsed apiResponse[tunnelConfigResponse]
				if err := json.Unmarshal(body, &parsed); err == nil && parsed.Success {
					for _, ing := range parsed.Result.Config.Ingress {
						if ing.Hostname == "" {
							continue
						}
						port, proto := parseIngressService(ing.Hostname, ing.Service)
						if port > 0 {
							addRoute(ing.Hostname, port, proto)
						}
					}
				}
			}
		}
	}

	// 2. Inspeccionar registros DNS CNAME apuntando a cfargotunnel.com en las zonas de la cuenta
	zonesToScan := []string{}
	if zoneID != "" {
		zonesToScan = append(zonesToScan, zoneID)
	}
	if listBody, err := c.doGet("https://api.cloudflare.com/client/v4/zones?per_page=50"); err == nil {
		var parsedZones apiResponse[[]Zone]
		if json.Unmarshal(listBody, &parsedZones) == nil && parsedZones.Success {
			for _, z := range parsedZones.Result {
				if accountID == "" || z.Account.ID == accountID {
					zonesToScan = append(zonesToScan, z.ID)
				}
			}
		}
	}

	visitedZones := make(map[string]bool)
	for _, zID := range zonesToScan {
		if zID == "" || visitedZones[zID] {
			continue
		}
		visitedZones[zID] = true
		dnsURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?per_page=100", zID)
		body, err := c.doGet(dnsURL)
		if err != nil {
			continue
		}
		var parsedDNS apiResponse[[]dnsRecord]
		if json.Unmarshal(body, &parsedDNS) == nil && parsedDNS.Success {
			for _, rec := range parsedDNS.Result {
				isTunnelCNAME := rec.Type == "CNAME" && strings.HasSuffix(strings.ToLower(rec.Content), ".cfargotunnel.com")
				if isTunnelCNAME || rec.Proxied {
					if port, proto := inferServicePortFromHostname(rec.Name); port > 0 {
						addRoute(rec.Name, port, proto)
					}
				}
			}
		}
	}

	return routes
}

func parseIngressService(hostname, service string) (int, string) {
	service = strings.TrimSpace(strings.ToLower(service))
	if strings.HasPrefix(service, "http_status:") {
		return 0, ""
	}
	if u, err := url.Parse(service); err == nil && u.Scheme != "" {
		proto := u.Scheme
		if pStr := u.Port(); pStr != "" {
			if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
				if proto == "tcp" || proto == "ssh" || proto == "rdp" || proto == "postgres" || proto == "postgresql" || proto == "mysql" {
					return p, "tcp"
				}
				if p == 5432 || p == 3306 || p == 6379 || p == 22 || p == 3389 || p == 1433 {
					return p, "tcp"
				}
			}
		}
		if proto == "ssh" {
			return 2222, "tcp"
		}
		if proto == "rdp" {
			return 3389, "tcp"
		}
	}
	return inferServicePortFromHostname(hostname)
}

func inferServicePortFromHostname(hostname string) (int, string) {
	h := strings.ToLower(hostname)
	sub := strings.Split(h, ".")[0]
	switch {
	case strings.Contains(sub, "db") || strings.Contains(sub, "postgres") || strings.Contains(sub, "pg") || strings.Contains(sub, "sql"):
		return 5432, "tcp"
	case strings.Contains(sub, "mysql") || strings.Contains(sub, "maria"):
		return 3306, "tcp"
	case strings.Contains(sub, "redis"):
		return 6379, "tcp"
	case strings.Contains(sub, "ssh"):
		return 2222, "tcp"
	case strings.Contains(sub, "rdp"):
		return 3389, "tcp"
	}
	return 0, ""
}
