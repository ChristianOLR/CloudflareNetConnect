package wireguard

import (
	"encoding/base64"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if priv == "" || pub == "" {
		t.Fatalf("expected non-empty keys, got priv=%q pub=%q", priv, pub)
	}

	privBytes, err := base64.StdEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("priv is not valid base64: %v", err)
	}
	if len(privBytes) != 32 {
		t.Fatalf("expected 32 bytes for private key, got %d", len(privBytes))
	}

	pubBytes, err := base64.StdEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("pub is not valid base64: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Fatalf("expected 32 bytes for public key, got %d", len(pubBytes))
	}

	hexKey := toHexKey(priv)
	if len(hexKey) != 64 {
		t.Fatalf("expected 64 hex characters, got %d", len(hexKey))
	}
}
