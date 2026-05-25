package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigureSetAndUpdate(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gcp-sso-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempConfigPath := filepath.Join(tempDir, "config.json")
	configPathOverride = tempConfigPath
	defer func() { configPathOverride = "" }()

	// Initialize empty config
	if err := SaveConfig(&Config{Profiles: make(map[string]Profile)}); err != nil {
		t.Fatalf("Failed to save empty config: %v", err)
	}

	// 1. Test creating a new profile
	setArgs := []string{
		"-account", "dev-user@example.com",
		"-project", "dev-project-123",
		"-region", "us-central1",
	}
	if err := handleConfigureSet("dev-profile", setArgs); err != nil {
		t.Errorf("Failed to create profile: %v", err)
	}

	// Verify it was saved
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	p, ok := cfg.Profiles["dev-profile"]
	if !ok {
		t.Fatal("Profile 'dev-profile' was not saved")
	}

	if p.Account != "dev-user@example.com" || p.Project != "dev-project-123" || p.Region != "us-central1" {
		t.Errorf("Saved profile fields do not match. Got: %+v", p)
	}

	// 2. Test updating only a single property
	updateArgs := []string{
		"-region", "europe-west1",
	}
	if err := handleConfigureSet("dev-profile", updateArgs); err != nil {
		t.Errorf("Failed to update profile: %v", err)
	}

	cfg, _ = LoadConfig()
	p = cfg.Profiles["dev-profile"]
	if p.Region != "europe-west1" {
		t.Errorf("Expected region to be updated to 'europe-west1', got: %s", p.Region)
	}
	// Check that other fields were preserved
	if p.Account != "dev-user@example.com" || p.Project != "dev-project-123" {
		t.Errorf("Other fields were modified during update: %+v", p)
	}

	// 3. Test adding GKE config
	gkeArgs := []string{
		"-gke-cluster", "dev-gke-1",
		"-gke-location", "us-central1-a",
	}
	if err := handleConfigureSet("dev-profile", gkeArgs); err != nil {
		t.Errorf("Failed to add GKE config: %v", err)
	}

	cfg, _ = LoadConfig()
	p = cfg.Profiles["dev-profile"]
	if p.GKE == nil {
		t.Fatal("GKE config was not created")
	}
	if p.GKE.Cluster != "dev-gke-1" || p.GKE.Location != "us-central1-a" {
		t.Errorf("GKE config mismatch: %+v", p.GKE)
	}
}

func TestConfigureSetValidations(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gcp-sso-config-test")
	defer os.RemoveAll(tempDir)
	configPathOverride = filepath.Join(tempDir, "config.json")
	defer func() { configPathOverride = "" }()

	// Initialize empty config
	SaveConfig(&Config{Profiles: make(map[string]Profile)})

	// Test creating new profile with missing project
	badArgs := []string{
		"-account", "user@example.com",
	}
	if err := handleConfigureSet("new-profile", badArgs); err == nil {
		t.Error("Expected error when creating new profile without project")
	}

	// Test creating new profile with missing account
	badArgs2 := []string{
		"-project", "project-123",
	}
	if err := handleConfigureSet("new-profile", badArgs2); err == nil {
		t.Error("Expected error when creating new profile without account")
	}
}

func TestConfigureDelete(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gcp-sso-config-test")
	defer os.RemoveAll(tempDir)
	configPathOverride = filepath.Join(tempDir, "config.json")
	defer func() { configPathOverride = "" }()

	// Save config with one profile
	initialCfg := &Config{
		Profiles: map[string]Profile{
			"delete-me": {
				Account: "user@example.com",
				Project: "project-123",
			},
		},
	}
	SaveConfig(initialCfg)

	// Test deleting non-existent profile
	if err := handleConfigureDelete("nonexistent"); err == nil {
		t.Error("Expected error when deleting non-existent profile")
	}

	// Test deleting existing profile
	if err := handleConfigureDelete("delete-me"); err != nil {
		t.Errorf("Failed to delete profile: %v", err)
	}

	// Verify config is empty
	cfg, _ := LoadConfig()
	if _, ok := cfg.Profiles["delete-me"]; ok {
		t.Error("Profile 'delete-me' was not deleted")
	}
}
