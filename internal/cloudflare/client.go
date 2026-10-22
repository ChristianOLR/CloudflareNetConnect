package cloudflare

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Account representa una organización de Cloudflare.
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Zone representa un dominio/zona de Cloudflare junto a su organización propietaria.
type Zone struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Account Account `json:"account"`
}

// Tunnel representa un túnel Zero Trust (cfd_tunnel).
type Tunnel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type apiResponse[T any] struct {
	Success  bool     `json:"success"`
	Errors   []any    `json:"errors"`
	Messages []string `json:"messages"`
	Result   T        `json:"result"`
}

// Client es el cliente HTTP ligero para la API v4 de Cloudflare.
type Client struct {
	apiToken   string
	httpClient *http.Client
}

// NewClient crea un nuevo cliente API para Cloudflare.
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) doGet(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error de red con API Cloudflare: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ListAccounts consulta las organizaciones asociadas al token.
func (c *Client) ListAccounts() ([]Account, error) {
	body, err := c.doGet("https://api.cloudflare.com/client/v4/accounts")
	if err != nil {
		return nil, err
	}
	var parsed apiResponse[[]Account]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("error parseando cuentas: %w", err)
	}
	if !parsed.Success {
		return nil, fmt.Errorf("error API Cloudflare: %v", parsed.Errors)
	}
	return parsed.Result, nil
}

// GetAccount consulta una cuenta específica por su ID.
func (c *Client) GetAccount(accountID string) (*Account, error) {
	body, err := c.doGet(fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s", accountID))
	if err != nil {
		return nil, err
	}
	var parsed apiResponse[Account]
	if err := json.Unmarshal(body, &parsed); err != nil || !parsed.Success {
		return nil, fmt.Errorf("no se pudo obtener cuenta %s", accountID)
	}
	return &parsed.Result, nil
}

// GetZone consulta los detalles de una zona por su ID (incluyendo nombre de cuenta y dominio).
func (c *Client) GetZone(zoneID string) (*Zone, error) {
	body, err := c.doGet(fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", zoneID))
	if err != nil {
		return nil, err
	}
	var parsed apiResponse[Zone]
	if err := json.Unmarshal(body, &parsed); err != nil || !parsed.Success {
		return nil, fmt.Errorf("no se pudo obtener zona %s", zoneID)
	}
	return &parsed.Result, nil
}

// ResolveAccountInfo resuelve dinámicamente el nombre e ID de la cuenta o zona sin valores hardcodeados.
func (c *Client) ResolveAccountInfo(accountID, zoneID string) (string, string) {
	if zoneID != "" {
		if z, err := c.GetZone(zoneID); err == nil && z != nil {
			accID := z.Account.ID
			if accID == "" {
				accID = accountID
			}
			if z.Account.Name != "" && z.Name != "" {
				return accID, fmt.Sprintf("%s (%s)", z.Account.Name, z.Name)
			}
			if z.Name != "" {
				return accID, z.Name
			}
		}
	}
	if accountID != "" {
		if acc, err := c.GetAccount(accountID); err == nil && acc != nil && acc.Name != "" {
			return acc.ID, acc.Name
		}
		if tunnels, err := c.ListTunnels(accountID); err == nil && len(tunnels) > 0 {
			short := accountID
			if len(short) > 6 {
				short = short[:6]
			}
			return accountID, fmt.Sprintf("Cloudflare [%s] (%s)", tunnels[0].Name, short)
		}
	}
	if accs, err := c.ListAccounts(); err == nil && len(accs) > 0 {
		return accs[0].ID, accs[0].Name
	}
	if accountID != "" {
		short := accountID
		if len(short) > 8 {
			short = short[:8]
		}
		return accountID, "Cuenta Cloudflare (" + short + ")"
	}
	return "cf-linked", "Cuenta Cloudflare Vinculada"
}

// ListTunnels lista los túneles Zero Trust existentes en una cuenta.
func (c *Client) ListTunnels(accountID string) ([]Tunnel, error) {
	body, err := c.doGet(fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel?is_deleted=false", accountID))
	if err != nil {
		return nil, err
	}
	var parsed apiResponse[[]Tunnel]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return parsed.Result, nil
}
