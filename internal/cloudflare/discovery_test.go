package cloudflare

import (
	"encoding/json"
	"testing"
)

func TestParseTeamnetRoutes(t *testing.T) {
	rawJSON := `{
		"success": true,
		"errors": [],
		"messages": [],
		"result": [
			{
				"id": "route-1",
				"network": "10.0.0.0/24",
				"tunnel_id": "tun-abc",
				"tunnel_name": "oficina-tunnel",
				"comment": "Red oficina"
			},
			{
				"id": "route-2",
				"network": "192.168.10.0/24",
				"tunnel_id": "tun-xyz",
				"tunnel_name": "datacenter",
				"deleted_at": "2026-01-01T00:00:00Z"
			}
		]
	}`

	var parsed apiResponse[[]TeamnetRoute]
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !parsed.Success || len(parsed.Result) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(parsed.Result))
	}

	activeCount := 0
	for _, r := range parsed.Result {
		if r.DeletedAt == nil {
			activeCount++
			if r.Network != "10.0.0.0/24" {
				t.Errorf("unexpected network: %s", r.Network)
			}
		}
	}
	if activeCount != 1 {
		t.Errorf("expected 1 active route, got %d", activeCount)
	}
}

func TestDiscoverTeamName(t *testing.T) {
	client := NewClient("dummy")
	team := client.DiscoverTeamName("", "Hispaserver (hispaserver.net)", "hispaserver.net")
	if team != "hispaserver" {
		t.Errorf("expected 'hispaserver', got '%s'", team)
	}

	team2 := client.DiscoverTeamName("", "OLR (olrcloud.com)", "olrcloud.com")
	if team2 != "olrcloud" {
		t.Errorf("expected 'olrcloud', got '%s'", team2)
	}
}

func TestAggregateCIDR(t *testing.T) {
	routesHostOnly := []TeamnetRoute{
		{Network: "192.168.1.3/32"},
		{Network: "192.168.1.4/32"},
		{Network: "192.168.1.250/32"},
	}
	agg := AggregateCIDR(routesHostOnly)
	if agg != "192.168.1.0/24" {
		t.Errorf("expected 192.168.1.0/24, got %s", agg)
	}

	routesWithSubnet := []TeamnetRoute{
		{Network: "192.168.1.3/32"},
		{Network: "10.50.0.0/16"},
	}
	aggSubnet := AggregateCIDR(routesWithSubnet)
	if aggSubnet != "10.50.0.0/16" {
		t.Errorf("expected 10.50.0.0/16, got %s", aggSubnet)
	}
}
