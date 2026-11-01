package wireguard

import (
	"net/url"
	"strings"
)

// ExtractZeroTrustToken extrae de forma segura el token JWT desde una URI com.cloudflare.warp:// o texto crudo.
func ExtractZeroTrustToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Caso 1: Viene con esquema com.cloudflare.warp://
	if strings.HasPrefix(raw, "com.cloudflare.warp://") {
		// Convertir temporalmente a https:// para parseo estándar de query params
		httpURL := "https://" + strings.TrimPrefix(raw, "com.cloudflare.warp://")
		if parsed, err := url.Parse(httpURL); err == nil {
			if token := parsed.Query().Get("token"); token != "" {
				return cleanJWT(token)
			}
		}
	}

	// Caso 2: Viene con parámetro token=
	if strings.Contains(raw, "token=") {
		parts := strings.Split(raw, "token=")
		if len(parts) > 1 {
			tokenPart := parts[1]
			if idx := strings.IndexAny(tokenPart, "& \t\r\n"); idx != -1 {
				tokenPart = tokenPart[:idx]
			}
			return cleanJWT(tokenPart)
		}
	}

	// Caso 3: Es directamente un JWT (3 segmentos separados por punto)
	return cleanJWT(raw)
}

func cleanJWT(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, `"'`)
	parts := strings.Split(token, ".")
	if len(parts) == 3 && len(parts[0]) > 5 && len(parts[1]) > 5 {
		return token
	}
	return ""
}
