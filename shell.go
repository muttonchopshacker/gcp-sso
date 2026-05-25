package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// handleShell spawns a subshell pre-configured with the profile's environment.
func handleShell(profileName string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		fmt.Printf("Error: Profile %q not found\n", profileName)
		os.Exit(1)
	}

	// 1. Check if authentication is bootstrapped
	needLogin := false
	
	// Check ADC cache
	adcPath, err := GetADCCachePath(profile.Account)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(adcPath); os.IsNotExist(err) {
		needLogin = true
	}

	// Check GKE kubeconfig (if configured)
	if profile.GKE != nil && profile.GKE.Cluster != "" {
		kubePath, err := GetKubeconfigPath(profileName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if _, err := os.Stat(kubePath); os.IsNotExist(err) {
			needLogin = true
		}
	}

	// 2. If not bootstrapped, run the login flow automatically
	if needLogin {
		fmt.Printf("Profile %q is not bootstrapped. Running login flow first...\n\n", profileName)
		if err := LoginProfile(profileName, cfg); err != nil {
			fmt.Printf("\nBootstrap failed: %v\n", err)
			os.Exit(1)
		}
		// Reload config after login in case GKE location or other details were dynamically merged
		cfg, _ = LoadConfig()
	}

	// 3. Get environment variables for the profile
	envMap, err := GetEnvMap(profileName, cfg)
	if err != nil {
		fmt.Printf("Error generating environment: %v\n", err)
		os.Exit(1)
	}

	// 4. Clean the parent environment (filter out existing GCP variables to prevent leaks)
	cleanedEnv := cleanParentEnv()

	// 5. Append the new profile environment variables
	for k, v := range envMap {
		cleanedEnv = append(cleanedEnv, fmt.Sprintf("%s=%s", k, v))
	}

	// 6. Set a custom prompt (PS1) to identify the subshell
	// Colors: blue bold prompt indicator
	customPrompt := fmt.Sprintf("PS1=[gcp-sso: %s] \\w \\$ ", profileName)
	cleanedEnv = append(cleanedEnv, customPrompt)

	// 7. Get user's default shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	fmt.Printf("Spawning subshell for profile %q using %s...\n", profileName, shell)
	fmt.Println("Type 'exit' or press Ctrl+D to return to your clean parent shell.")
	fmt.Println("-----------------------------------------------------------------")

	// 8. Spawn interactive subprocess
	cmd := exec.Command(shell)
	cmd.Env = cleanedEnv
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("\nSubshell execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n-----------------------------------------------------------------")
	fmt.Printf("Returned to clean parent shell. Profile %q deactivated.\n", profileName)
}

// cleanParentEnv filters out any existing GCP and Kubernetes session environment variables
// inherited from the parent shell to ensure a clean isolated start.
func cleanParentEnv() []string {
	var cleaned []string
	prefixesToFilter := []string{
		"CLOUDSDK_",
		"GOOGLE_",
		"GCP_SSO_",
		"KUBECONFIG",
	}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		filter := false
		for _, prefix := range prefixesToFilter {
			if strings.HasPrefix(key, prefix) {
				filter = true
				break
			}
		}

		if !filter {
			cleaned = append(cleaned, env)
		}
	}
	return cleaned
}
