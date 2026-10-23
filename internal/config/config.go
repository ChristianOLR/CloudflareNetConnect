package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// TunnelRoute define el mapeo local hacia un túnel de Cloudflare.
type TunnelRoute struct {
	Hostname  string `json:"hostname"`
	LocalPort int    `json:"local_port"`
	Protocol  string `json:"protocol"` // "tcp" o "http"
	Enabled   bool   `json:"enabled"`
}

// AccountConfig contiene las credenciales y rutas por organización.
type AccountConfig struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	AccountID string        `json:"account_id"`
	APIToken  string        `json:"api_token"`
	Routes    []TunnelRoute `json:"routes"`
	Enabled   bool          `json:"enabled"`
}

// AppConfig es la configuración global de CloudflareNetConnect.
type AppConfig struct {
	MasterToggle bool            `json:"master_toggle"`
	Accounts     []AccountConfig `json:"accounts"`
}

// DefaultConfig genera una configuración inicial limpia sin cuentas predefinidas.
func DefaultConfig() *AppConfig {
	return &AppConfig{
		MasterToggle: true,
		Accounts:     []AccountConfig{},
	}
}

// DefaultConfigPath devuelve la ruta adecuada según la plataforma y permisos.
func DefaultConfigPath(baseDir string) string {
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		return filepath.Join(baseDir, "config", "netconnect.json")
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "cloudflarenetconnect", "netconnect.json")
}

// DefaultLogPath devuelve la ruta adecuada de logs con permisos de usuario.
func DefaultLogPath(baseDir string) string {
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		return filepath.Join(baseDir, "netconnect.log")
	}
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, "Library", "Logs", "CloudflareNetConnect")
	_ = os.MkdirAll(logDir, 0755)
	return filepath.Join(logDir, "netconnect.log")
}

// LoadConfig carga el archivo JSON desde la ruta dada o crea el default.
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := DefaultConfig()
		_ = SaveConfig(path, cfg)
		return cfg, nil
	} else if err != nil {
		return nil, err
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig guarda la configuración de manera segura en disco.
func SaveConfig(path string, cfg *AppConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
