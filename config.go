package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GKEConfig defines the GKE cluster configuration for a profile.
type GKEConfig struct {
	Cluster  string `json:"cluster,omitempty"`
	Location string `json:"location,omitempty"` // regional or zonal
}

// Profile defines a GCP SSO profile.
type Profile struct {
	Account                   string     `json:"account"`
	Project                   string     `json:"project"`
	Region                    string     `json:"region,omitempty"`
	Zone                      string     `json:"zone,omitempty"`
	ImpersonateServiceAccount string     `json:"impersonate_service_account,omitempty"`
	GKE                       *GKEConfig `json:"gke,omitempty"`
}

// Config defines the structure of the config.json file.
type Config struct {
	Profiles map[string]Profile `json:"profiles"`
}

// GetConfigDir returns the path to the configuration directory (~/.config/gcp-sso).
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "gcp-sso"), nil
}

// GetConfigPath returns the path to config.json.
func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig loads the configuration from config.json.
func LoadConfig() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &Config{Profiles: make(map[string]Profile)}, nil
		}
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config JSON: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to config.json.
func SaveConfig(cfg *Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config JSON: %w", err)
	}

	return nil
}
