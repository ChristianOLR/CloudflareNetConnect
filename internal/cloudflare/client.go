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

// ListAccounts consulta las organizaciones asociadas al token.
func (c *Client) ListAccounts() ([]Account, error) {
	url := "https://api.cloudflare.com/client/v4/accounts"
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

	body, err := io.ReadAll(resp.Body)
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

// ListTunnels lista los túneles Zero Trust existentes en una cuenta.
func (c *Client) ListTunnels(accountID string) ([]Tunnel, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel?is_deleted=false", accountID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed apiResponse[[]Tunnel]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	return parsed.Result, nil
}
