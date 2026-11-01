package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSaveWithPrivateNetwork(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cfnetconnect_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.json")
	cfg := &AppConfig{
		MasterToggle: true,
		Accounts: []AccountConfig{
			{
				ID:        "acc-123",
				Name:      "Test Account",
				AccountID: "cf-acc-1",
				APIToken:  "dummy-token",
				Enabled:   true,
				PrivateNetwork: &PrivateNetworkConfig{
					Enabled:    true,
					AutoDetect: true,
					CIDR:       "192.168.1.0/24",
				},
				Routes: []TunnelRoute{
					{
						Hostname:  "app.example.com",
						LocalPort: 8080,
						Protocol:  "http",
						Enabled:   true,
					},
				},
			},
		},
	}

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(loaded.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(loaded.Accounts))
	}
	acc := loaded.Accounts[0]
	if acc.PrivateNetwork == nil {
		t.Fatalf("expected PrivateNetwork not nil")
	}
	if !acc.PrivateNetwork.Enabled || !acc.PrivateNetwork.AutoDetect || acc.PrivateNetwork.CIDR != "192.168.1.0/24" {
		t.Errorf("unexpected PrivateNetwork values: %+v", acc.PrivateNetwork)
	}
}
