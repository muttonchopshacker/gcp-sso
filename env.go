package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetProfileDir returns the path to the profile-specific root directory (~/.config/gcp-sso/profiles/<profile>).
func GetProfileDir(profileName string) (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	configDir := filepath.Dir(configPath)
	return filepath.Join(configDir, "profiles", profileName), nil
}

// GetKubeconfigPath returns the path to the profile-specific kubeconfig file.
func GetKubeconfigPath(profileName string) (string, error) {
	profileDir, err := GetProfileDir(profileName)
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, "kube.yaml"), nil
}

// GetCloudsdkConfigDir returns the path to the profile-specific gcloud config directory.
func GetCloudsdkConfigDir(profileName string) (string, error) {
	profileDir, err := GetProfileDir(profileName)
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, "gcloud"), nil
}

// GetADCCachePath returns the path to the cached ADC file for a given profile.
func GetADCCachePath(profileName string) (string, error) {
	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return "", err
	}
	return filepath.Join(gcloudDir, "application_default_credentials.json"), nil
}

// GetEnvMap generates a map of all environment variables to be set for a profile.
func GetEnvMap(profileName string, cfg *Config) (map[string]string, error) {
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", profileName)
	}

	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return nil, err
	}

	env := map[string]string{
		"GCP_SSO_PROFILE":      profileName,
		"CLOUDSDK_CONFIG":      gcloudDir,
		"CLOUDSDK_CORE_PROJECT": profile.Project,
		"CLOUDSDK_CORE_ACCOUNT": profile.Account,
	}

	if profile.Region != "" {
		env["CLOUDSDK_COMPUTE_REGION"] = profile.Region
	}
	if profile.Zone != "" {
		env["CLOUDSDK_COMPUTE_ZONE"] = profile.Zone
	}

	if profile.ImpersonateServiceAccount != "" {
		env["CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT"] = profile.ImpersonateServiceAccount
		env["GOOGLE_IMPERSONATE_SERVICE_ACCOUNT"] = profile.ImpersonateServiceAccount
	}

	adcPath, err := GetADCCachePath(profileName)
	if err == nil {
		if _, err := os.Stat(adcPath); err == nil {
			env["GOOGLE_APPLICATION_CREDENTIALS"] = adcPath
		}
	}

	if profile.GKE != nil && profile.GKE.Cluster != "" {
		kubePath, err := GetKubeconfigPath(profileName)
		if err == nil {
			env["KUBECONFIG"] = kubePath
		}
	}

	return env, nil
}

// GenerateEnvOutputs generates shell export commands for the specified profile.
// This prints exports for active values, and unsets for missing values to clean previous states.
func GenerateEnvOutputs(profileName string, cfg *Config) error {
	env, err := GetEnvMap(profileName, cfg)
	if err != nil {
		return err
	}

	// Known set of variables we manage
	keys := []string{
		"GCP_SSO_PROFILE",
		"CLOUDSDK_CONFIG",
		"CLOUDSDK_CORE_PROJECT",
		"CLOUDSDK_CORE_ACCOUNT",
		"CLOUDSDK_COMPUTE_REGION",
		"CLOUDSDK_COMPUTE_ZONE",
		"CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT",
		"GOOGLE_IMPERSONATE_SERVICE_ACCOUNT",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"KUBECONFIG",
	}

	for _, key := range keys {
		val, exists := env[key]
		if exists {
			fmt.Printf("export %s=%q\n", key, val)
		} else {
			// Handle printing comments/warnings for missing required files
			if key == "GOOGLE_APPLICATION_CREDENTIALS" {
				fmt.Printf("# WARNING: ADC cache not found. Run 'gcp-sso login %s' to authenticate.\n", profileName)
			} else if key == "KUBECONFIG" {
				profile := cfg.Profiles[profileName]
				if profile.GKE != nil && profile.GKE.Cluster != "" {
					fmt.Printf("# WARNING: Kubeconfig not found for GKE cluster %s. Run 'gcp-sso login %s' to configure.\n", profile.GKE.Cluster, profileName)
				}
			}
			fmt.Printf("unset %s\n", key)
		}
	}

	return nil
}
