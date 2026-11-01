package cloudflare

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// TeamnetRoute representa una ruta de subred privada en Cloudflare Zero Trust.
type TeamnetRoute struct {
	ID         string  `json:"id"`
	Network    string  `json:"network"` // CIDR, ej. "192.168.1.0/24"
	TunnelID   string  `json:"tunnel_id"`
	TunnelName string  `json:"tunnel_name,omitempty"`
	Comment    string  `json:"comment,omitempty"`
	DeletedAt  *string `json:"deleted_at,omitempty"`
}

// ListTeamnetRoutes consulta las subredes privadas configuradas en la organización.
func (c *Client) ListTeamnetRoutes(accountID string) ([]TeamnetRoute, error) {
	if accountID == "" {
		return nil, fmt.Errorf("accountID no puede estar vacío")
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/teamnet/routes?is_deleted=false", accountID)
	body, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	var parsed apiResponse[[]TeamnetRoute]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("error parseando rutas teamnet: %w", err)
	}
	if !parsed.Success {
		return nil, fmt.Errorf("error API teamnet: %v", parsed.Errors)
	}

	active := make([]TeamnetRoute, 0, len(parsed.Result))
	for _, r := range parsed.Result {
		if r.DeletedAt == nil && strings.TrimSpace(r.Network) != "" {
			active = append(active, r)
		}
	}
	return active, nil
}

// DiscoverPrivateCIDR busca dinámicamente el CIDR de red privada configurado en Zero Trust.
func (c *Client) DiscoverPrivateCIDR(accountID, zoneID string) (string, bool) {
	routes, err := c.ListTeamnetRoutes(accountID)
	if err != nil {
		log.Printf("[DISCOVERY] Error consultando rutas privadas teamnet: %v\n", err)
		return "", false
	}
	if len(routes) == 0 {
		return "", false
	}

	// Si hay túneles asociados a esta zona, buscar coincidencia específica
	if zoneID != "" {
		if z, err := c.GetZone(zoneID); err == nil && z != nil && z.Name != "" {
			domain := strings.ToLower(strings.TrimSpace(z.Name))
			for _, r := range routes {
				cComment := strings.ToLower(r.Comment)
				cTunnel := strings.ToLower(r.TunnelName)
				if strings.Contains(cComment, domain) || strings.Contains(cTunnel, domain) {
					return strings.TrimSpace(r.Network), true
				}
			}
		}
	}

	// Devolver la primera subred activa de la cuenta
	return strings.TrimSpace(routes[0].Network), true
}
