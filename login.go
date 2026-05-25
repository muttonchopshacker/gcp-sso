package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IsAccountLoggedIn checks if the account is logged in and its session is still active in the profile context.
func IsAccountLoggedIn(profileName string, account string) bool {
	// Attempt to print the access token which actually performs validation against the Google token server
	cmd := exec.Command("gcloud", "auth", "print-access-token", "--account="+account)
	
	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err == nil {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcloudDir)
	}

	// If print-access-token succeeds, we are logged in and session is valid
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// IsADCActive checks if the Application Default Credentials are active and not expired for the profile.
func IsADCActive(profileName string) bool {
	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return false
	}

	cmd := exec.Command("gcloud", "auth", "application-default", "print-access-token")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcloudDir)

	// If print-access-token succeeds, ADC is active and valid
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

type TokenInfo struct {
	ExpiresIn int `json:"expires_in"`
}

// GetTokenTTL returns the remaining TTL of the token (gcloud or ADC) in a human-readable format.
func GetTokenTTL(profileName string, isADC bool) string {
	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return "Error"
	}

	var cmd *exec.Cmd
	if isADC {
		cmd = exec.Command("gcloud", "auth", "application-default", "print-access-token")
	} else {
		cmd = exec.Command("gcloud", "auth", "print-access-token")
	}

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcloudDir)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "Expired/Inactive"
	}

	token := strings.TrimSpace(out.String())
	if token == "" {
		return "Expired/Inactive"
	}

	// Query TokenInfo endpoint using a standard client with a timeout
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=" + token)
	if err != nil {
		return "Active (Offline)"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Invalid/Expired"
	}

	var info TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "Active"
	}

	if info.ExpiresIn <= 0 {
		return "Expired"
	}

	duration := time.Duration(info.ExpiresIn) * time.Second
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

// RunGcloudLogin runs 'gcloud auth login' inside the isolated profile directory.
func RunGcloudLogin(profileName string, account string) error {
	fmt.Printf("Authenticating gcloud for account: %s...\n", account)
	
	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(gcloudDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cmd := exec.Command("gcloud", "auth", "login", "--account="+account)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Force writing credentials to the isolated profile folder
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcloudDir)

	return cmd.Run()
}

// BootstrapADC authenticates ADC inside the profile's custom config directory.
func BootstrapADC(profileName string, account string) error {
	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return err
	}

	// Create profile directory for config if it doesn't exist
	if err := os.MkdirAll(gcloudDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	fmt.Printf("Authenticating Application Default Credentials (ADC) for %s...\n", account)
	fmt.Println("Please complete the login in your browser. Make sure to select the correct account!")
	
	cmd := exec.Command("gcloud", "auth", "application-default", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Force writing ADC directly inside the profile's folder
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcloudDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run gcloud auth application-default login: %w", err)
	}

	adcPath, _ := GetADCCachePath(profileName)
	fmt.Printf("ADC cached successfully at %s\n", adcPath)

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

	gcloudDir, err := GetCloudsdkConfigDir(profileName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(kubePath), 0755); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}

	fmt.Printf("Fetching GKE credentials for cluster %s in project %s...\n", profile.GKE.Cluster, profile.Project)

	args := []string{
		"container", "clusters", "get-credentials", profile.GKE.Cluster,
	}
	
	if profile.GKE.Location != "" {
		parts := strings.Split(profile.GKE.Location, "-")
		if len(parts) == 3 {
			args = append(args, "--zone="+profile.GKE.Location)
		} else if len(parts) == 2 {
			args = append(args, "--region="+profile.GKE.Location)
		} else if profile.GKE.Location != "" {
			args = append(args, "--zone="+profile.GKE.Location)
		}
	}

	cmd := exec.Command("gcloud", args...)
	
	// Set up isolated environment for this command
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CLOUDSDK_CONFIG="+gcloudDir) // Use isolated gcloud context
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

	// 1. Ensure gcloud is logged in and session is active
	if !IsAccountLoggedIn(profileName, profile.Account) {
		if err := RunGcloudLogin(profileName, profile.Account); err != nil {
			return fmt.Errorf("gcloud authentication failed: %w", err)
		}
	} else {
		fmt.Printf("gcloud already authenticated and active for %s\n", profile.Account)
	}

	// 2. Ensure ADC is active
	if !IsADCActive(profileName) {
		if err := BootstrapADC(profileName, profile.Account); err != nil {
			return fmt.Errorf("ADC bootstrapping failed: %w", err)
		}
	} else {
		fmt.Printf("ADC already authenticated and active for %s\n", profile.Account)
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

// LogoutProfile deletes all cached credentials and GKE config for a profile.
func LogoutProfile(profileName string, cfg *Config) error {
	if _, ok := cfg.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	profileDir, err := GetProfileDir(profileName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		fmt.Printf("Profile %q is already logged out (no credentials found).\n", profileName)
		return nil
	}

	fmt.Printf("Logging out of profile %q and cleaning up credentials...\n", profileName)
	if err := os.RemoveAll(profileDir); err != nil {
		return fmt.Errorf("failed to delete profile directory: %w", err)
	}

	fmt.Printf("Successfully logged out of profile %q.\n", profileName)
	return nil
}
