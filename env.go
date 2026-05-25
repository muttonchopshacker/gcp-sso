package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetKubeconfigPath returns the path to the profile-specific kubeconfig file.
func GetKubeconfigPath(profileName string) (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kube", fmt.Sprintf("%s.yaml", profileName)), nil
}

// GetADCCachePath returns the path to the cached ADC file for a given account.
func GetADCCachePath(account string) (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "adc", fmt.Sprintf("%s.json", account)), nil
}

// GenerateEnvOutputs generates shell export commands for the specified profile.
func GenerateEnvOutputs(profileName string, cfg *Config) error {
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	fmt.Printf("export GCP_SSO_PROFILE=%q\n", profileName)
	fmt.Printf("export CLOUDSDK_CORE_PROJECT=%q\n", profile.Project)
	fmt.Printf("export CLOUDSDK_CORE_ACCOUNT=%q\n", profile.Account)

	if profile.Region != "" {
		fmt.Printf("export CLOUDSDK_COMPUTE_REGION=%q\n", profile.Region)
	} else {
		fmt.Println("unset CLOUDSDK_COMPUTE_REGION")
	}

	if profile.Zone != "" {
		fmt.Printf("export CLOUDSDK_COMPUTE_ZONE=%q\n", profile.Zone)
	} else {
		fmt.Println("unset CLOUDSDK_COMPUTE_ZONE")
	}

	// Handle Impersonation
	if profile.ImpersonateServiceAccount != "" {
		fmt.Printf("export CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT=%q\n", profile.ImpersonateServiceAccount)
		fmt.Printf("export GOOGLE_IMPERSONATE_SERVICE_ACCOUNT=%q\n", profile.ImpersonateServiceAccount)
	} else {
		fmt.Println("unset CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT")
		fmt.Println("unset GOOGLE_IMPERSONATE_SERVICE_ACCOUNT")
	}

	// Handle ADC
	adcPath, err := GetADCCachePath(profile.Account)
	if err == nil {
		if _, err := os.Stat(adcPath); err == nil {
			fmt.Printf("export GOOGLE_APPLICATION_CREDENTIALS=%q\n", adcPath)
		} else {
			fmt.Printf("# WARNING: ADC cache not found for account %s. Run 'gcp-sso login %s' to authenticate.\n", profile.Account, profileName)
			fmt.Println("unset GOOGLE_APPLICATION_CREDENTIALS")
		}
	}

	// Handle GKE / Kubeconfig
	if profile.GKE != nil && profile.GKE.Cluster != "" {
		kubePath, err := GetKubeconfigPath(profileName)
		if err == nil {
			fmt.Printf("export KUBECONFIG=%q\n", kubePath)
			if _, err := os.Stat(kubePath); err != nil {
				fmt.Printf("# WARNING: Kubeconfig not found for GKE cluster %s. Run 'gcp-sso login %s' to configure.\n", profile.GKE.Cluster, profileName)
			}
		}
	} else {
		fmt.Println("unset KUBECONFIG")
	}

	return nil
}
