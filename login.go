package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsAccountLoggedIn checks if the account is already logged in to gcloud.
func IsAccountLoggedIn(account string) bool {
	cmd := exec.Command("gcloud", "auth", "list", "--format=value(account)", "--filter=status:ACTIVE")
	// We also want to check all accounts, not just active one, because CLOUDSDK_CORE_ACCOUNT can switch active one.
	cmd = exec.Command("gcloud", "auth", "list", "--format=value(account)")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}

	accounts := strings.Split(out.String(), "\n")
	for _, acc := range accounts {
		if strings.TrimSpace(acc) == account {
			return true
		}
	}
	return false
}

// RunGcloudLogin runs 'gcloud auth login' for the specified account.
func RunGcloudLogin(account string) error {
	fmt.Printf("Authenticating gcloud for account: %s...\n", account)
	cmd := exec.Command("gcloud", "auth", "login", "--account="+account)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// BootstrapADC authenticates ADC for the account and caches it.
func BootstrapADC(account string) error {
	adcCachePath, err := GetADCCachePath(account)
	if err != nil {
		return err
	}

	// Create parent directory for cache if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(adcCachePath), 0755); err != nil {
		return fmt.Errorf("failed to create ADC cache directory: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	globalADCPath := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	backupADCPath := globalADCPath + ".backup"

	// 1. Backup global ADC if it exists
	hasBackup := false
	if _, err := os.Stat(globalADCPath); err == nil {
		fmt.Println("Backing up existing global Application Default Credentials...")
		if err := os.Rename(globalADCPath, backupADCPath); err != nil {
			return fmt.Errorf("failed to backup global ADC: %w", err)
		}
		hasBackup = true
	}

	// Defer restoration of backup
	defer func() {
		if hasBackup {
			fmt.Println("Restoring global Application Default Credentials backup...")
			os.Rename(backupADCPath, globalADCPath)
		}
	}()

	// 2. Run gcloud auth application-default login
	fmt.Printf("Authenticating Application Default Credentials (ADC) for %s...\n", account)
	fmt.Println("Please complete the login in your browser. Make sure to select the correct account!")
	
	// We pass --no-launch-browser to make it easier to see the link if needed,
	// but standard login is usually better. Let's just run standard login.
	cmd := exec.Command("gcloud", "auth", "application-default", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run gcloud auth application-default login: %w", err)
	}

	// 3. Copy generated ADC to cache
	if _, err := os.Stat(globalADCPath); err != nil {
		return fmt.Errorf("expected ADC file at %s was not created: %w", globalADCPath, err)
	}

	// Read and write to copy file
	data, err := os.ReadFile(globalADCPath)
	if err != nil {
		return fmt.Errorf("failed to read generated ADC: %w", err)
	}

	if err := os.WriteFile(adcCachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to cache ADC: %w", err)
	}

	fmt.Printf("ADC cached successfully at %s\n", adcCachePath)

	// Clean up the generated global ADC so we can restore backup
	os.Remove(globalADCPath)

	return nil
}

// BootstrapGKE configures GKE credentials in an isolated kubeconfig file.
func BootstrapGKE(profileName string, profile Profile) error {
	if profile.GKE == nil || profile.GKE.Cluster == "" {
		return nil
	}

	kubePath, err := GetKubeconfigPath(profileName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(kubePath), 0755); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}

	// Remove existing kubeconfig if it's there to start fresh, or we can let gcloud merge it.
	// Let's let gcloud merge it or create it.

	fmt.Printf("Fetching GKE credentials for cluster %s in project %s...\n", profile.GKE.Cluster, profile.Project)

	args := []string{
		"container", "clusters", "get-credentials", profile.GKE.Cluster,
	}
	
	if profile.GKE.Location != "" {
		// GKE locations can be regional or zonal. gcloud handles both.
		// But we need to specify --zone or --region or --location.
		// gcloud supports --region or --zone.
		// Actually, standard practice is to use --region or --zone.
		// Let's check if we can use --location or if we need to detect.
		// Usually, GKE get-credentials supports --region and --zone.
		// Let's try to guess if it's regional (e.g. us-central1) or zonal (e.g. us-central1-a).
		// Zonal usually has 3 segments (provider-region-zone), regional has 2.
		// But to be safe, we can just try to pass --location if supported, or check the format.
		// Actually, gcloud get-credentials supports both --region and --zone.
		// Let's assume if the location has 3 segments (2 hyphens) it is zonal, else regional.
		// Example: us-central1 (regional), us-central1-a (zonal).
		parts := strings.Split(profile.GKE.Location, "-")
		if len(parts) == 3 {
			args = append(args, "--zone="+profile.GKE.Location)
		} else if len(parts) == 2 {
			args = append(args, "--region="+profile.GKE.Location)
		} else if profile.GKE.Location != "" {
			// Fallback, just try --zone
			args = append(args, "--zone="+profile.GKE.Location)
		}
	}

	cmd := exec.Command("gcloud", args...)
	
	// Set up isolated environment for this command
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "KUBECONFIG="+kubePath)
	cmd.Env = append(cmd.Env, "CLOUDSDK_CORE_PROJECT="+profile.Project)
	cmd.Env = append(cmd.Env, "CLOUDSDK_CORE_ACCOUNT="+profile.Account)
	
	if profile.ImpersonateServiceAccount != "" {
		cmd.Env = append(cmd.Env, "CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT="+profile.ImpersonateServiceAccount)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch GKE credentials: %w", err)
	}

	fmt.Printf("Kubeconfig generated successfully at %s\n", kubePath)
	return nil
}

// LoginProfile bootstraps authentication for a profile.
func LoginProfile(profileName string, cfg *Config) error {
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	// 1. Ensure gcloud is logged in
	if !IsAccountLoggedIn(profile.Account) {
		if err := RunGcloudLogin(profile.Account); err != nil {
			return fmt.Errorf("gcloud authentication failed: %w", err)
		}
	} else {
		fmt.Printf("gcloud already authenticated for %s\n", profile.Account)
	}

	// 2. Ensure ADC is cached
	adcPath, err := GetADCCachePath(profile.Account)
	if err != nil {
		return err
	}
	
	if _, err := os.Stat(adcPath); os.IsNotExist(err) {
		if err := BootstrapADC(profile.Account); err != nil {
			return fmt.Errorf("ADC bootstrapping failed: %w", err)
		}
	} else {
		fmt.Printf("ADC already cached for %s\n", profile.Account)
		// Allow force re-auth? Maybe we can have a --force flag.
		// For now, we can just assume it's fine, or let them delete the cache file.
	}

	// 3. Ensure GKE is configured
	if profile.GKE != nil && profile.GKE.Cluster != "" {
		if err := BootstrapGKE(profileName, profile); err != nil {
			return fmt.Errorf("GKE bootstrapping failed: %w", err)
		}
	}

	fmt.Printf("\nSuccess! Profile %q is ready.\n", profileName)
	fmt.Printf("To activate, run: eval $(gcp-sso env %s)\n", profileName)

	return nil
}
