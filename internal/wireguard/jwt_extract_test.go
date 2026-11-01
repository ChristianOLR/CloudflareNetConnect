package wireguard

import (
	"testing"
)

func TestExtractZeroTrustToken(t *testing.T) {
	fakeJWT := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Warp URI",
			input:    "com.cloudflare.warp://hispaserver.cloudflareaccess.com/auth?token=" + fakeJWT,
			expected: fakeJWT,
		},
		{
			name:     "Query param alone",
			input:    "auth?token=" + fakeJWT + "&other=123",
			expected: fakeJWT,
		},
		{
			name:     "Raw JWT",
			input:    fakeJWT,
			expected: fakeJWT,
		},
		{
			name:     "Invalid string",
			input:    "not.a.valid",
			expected: "",
		},
		{
			name:     "Empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractZeroTrustToken(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractZeroTrustToken(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
