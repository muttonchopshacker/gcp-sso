package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gcp-sso-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set the override path
	tempConfigPath := filepath.Join(tempDir, "config.json")
	configPathOverride = tempConfigPath
	defer func() { configPathOverride = "" }() // Reset after test

	// Define a test configuration
	testCfg := &Config{
		Profiles: map[string]Profile{
			"test-dev": {
				Account: "dev-user@example.com",
				Project: "dev-project-id",
				Region:  "us-central1",
				Zone:    "us-central1-a",
			},
			"test-prod": {
				Account:                   "prod-admin@example.com",
				Project:                   "prod-project-id",
				ImpersonateServiceAccount: "prod-sa@prod-project-id.iam.gserviceaccount.com",
				GKE: &GKEConfig{
					Cluster:  "prod-cluster",
					Location: "us-central1",
				},
			},
		},
	}

	// 1. Save the config
	if err := SaveConfig(testCfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify the file was actually created
	if _, err := os.Stat(tempConfigPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at expected path: %s", tempConfigPath)
	}

	// 2. Load the config back
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// 3. Verify the loaded config matches the original
	if !reflect.DeepEqual(testCfg, loadedCfg) {
		t.Errorf("Loaded config does not match original.\nOriginal: %+v\nLoaded:   %+v", testCfg, loadedCfg)
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Set override to a path that definitely doesn't exist
	configPathOverride = "/path/to/definitely/nonexistent/config.json"
	defer func() { configPathOverride = "" }()

	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed on non-existent file: %v", err)
	}

	if loadedCfg == nil {
		t.Fatal("Expected non-nil Config even if file doesn't exist")
	}

	if len(loadedCfg.Profiles) != 0 {
		t.Errorf("Expected empty profiles map, got %d profiles", len(loadedCfg.Profiles))
	}
}
