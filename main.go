package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"text/tabwriter"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		handleInit()
	case "list":
		handleList()
	case "status":
		handleStatus()
	case "login":
		if len(os.Args) < 3 {
			fmt.Println("Error: Profile name required. Usage: gcp-sso login <profile-name>")
			os.Exit(1)
		}
		handleLogin(os.Args[2])
	case "env":
		if len(os.Args) < 3 {
			fmt.Println("Error: Profile name required. Usage: gcp-sso env <profile-name>")
			os.Exit(1)
		}
		handleEnv(os.Args[2])
	case "console":
		profileName := ""
		if len(os.Args) >= 3 {
			profileName = os.Args[2]
		}
		handleConsole(profileName)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command %q. Use 'gcp-sso help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("GCP SSO CLI (gcp-sso) - Manage Google Cloud Profiles & Sessions")
	fmt.Println("\nUsage:")
	fmt.Println("  gcp-sso <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  init               Initialize config directory and create config.json template")
	fmt.Println("  list               List all configured profiles")
	fmt.Println("  status             Show current active profile (from environment)")
	fmt.Println("  login <profile>    Authenticate and bootstrap a profile (gcloud + ADC + GKE)")
	fmt.Println("  env <profile>      Generate shell environment exports (intended for eval)")
	fmt.Println("  console [<profile>] Open Google Cloud Console for active or specified profile")
	fmt.Println("  help               Show this help message")
	fmt.Println("\nFor shell integration, add the following to your shell profile (e.g. ~/.bashrc or ~/.zshrc):")
	fmt.Print(`
gsp() {
  if [ $# -eq 0 ]; then
    echo "Usage: gsp <profile-name>  (or 'gsp off' to deactivate)"
    return 1
  fi
  local profile="$1"
  if [ "$profile" = "unset" ] || [ "$profile" = "off" ]; then
    unset GCP_SSO_PROFILE
    unset CLOUDSDK_CORE_PROJECT
    unset CLOUDSDK_CORE_ACCOUNT
    unset CLOUDSDK_COMPUTE_REGION
    unset CLOUDSDK_COMPUTE_ZONE
    unset CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT
    unset GOOGLE_IMPERSONATE_SERVICE_ACCOUNT
    unset GOOGLE_APPLICATION_CREDENTIALS
    unset KUBECONFIG
    echo "GCP Profile cleared."
  else
    local env_output
    env_output=$(gcp-sso env "$profile")
    if [ $? -eq 0 ]; then
      eval "$env_output"
      echo "Switched to GCP Profile: $profile"
    fi
  fi
}
`)
}

func handleInit() {
	path, err := GetConfigPath()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Config file already exists at %s. Skipping initialization.\n", path)
		return
	}

	// Create default config with examples
	defaultCfg := &Config{
		Profiles: map[string]Profile{
			"example-dev": {
				Account: "user@company.com",
				Project: "my-dev-project",
				Region:  "us-central1",
				Zone:    "us-central1-a",
			},
			"example-prod": {
				Account:                   "admin@company.com",
				Project:                   "my-prod-project",
				ImpersonateServiceAccount: "admin-sa@my-prod-project.iam.gserviceaccount.com",
				GKE: &GKEConfig{
					Cluster:  "prod-gke-cluster",
					Location: "us-central1",
				},
			},
		},
	}

	if err := SaveConfig(defaultCfg); err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully initialized configuration directory.\nTemplate config created at %s\n", path)
	fmt.Println("Please edit this file to add your own profiles.")
}

func handleList() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured. Run 'gcp-sso init' to create a template config.")
		return
	}

	activeProfile := os.Getenv("GCP_SSO_PROFILE")

	// Use tabwriter for clean column formatting
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ACTIVE\tNAME\tACCOUNT\tPROJECT\tIMPERSONATE SA\tGKE CLUSTER")
	fmt.Fprintln(w, "------\t----\t-------\t-------\t--------------\t-----------")

	for name, p := range cfg.Profiles {
		activeMarker := ""
		if name == activeProfile {
			activeMarker = "*"
		}

		impersonate := p.ImpersonateServiceAccount
		if impersonate == "" {
			impersonate = "-"
		}

		gke := "-"
		if p.GKE != nil && p.GKE.Cluster != "" {
			gke = p.GKE.Cluster
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", activeMarker, name, p.Account, p.Project, impersonate, gke)
	}
	w.Flush()
}

func handleStatus() {
	activeProfile := os.Getenv("GCP_SSO_PROFILE")
	if activeProfile == "" {
		fmt.Println("No active GCP Profile in this shell session.")
		fmt.Println("Use 'gsp <profile-name>' to activate a profile.")
		return
	}

	fmt.Printf("Active Profile:    %s\n", activeProfile)
	fmt.Printf("Project:           %s\n", os.Getenv("CLOUDSDK_CORE_PROJECT"))
	fmt.Printf("Account:           %s\n", os.Getenv("CLOUDSDK_CORE_ACCOUNT"))
	
	if reg := os.Getenv("CLOUDSDK_COMPUTE_REGION"); reg != "" {
		fmt.Printf("Region:            %s\n", reg)
	}
	if zone := os.Getenv("CLOUDSDK_COMPUTE_ZONE"); zone != "" {
		fmt.Printf("Zone:              %s\n", zone)
	}

	if imp := os.Getenv("CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT"); imp != "" {
		fmt.Printf("Impersonating SA:  %s\n", imp)
	}

	if adc := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); adc != "" {
		fmt.Printf("ADC File:          %s\n", adc)
	}

	if kube := os.Getenv("KUBECONFIG"); kube != "" {
		fmt.Printf("Kubeconfig:        %s\n", kube)
	}
}

func handleLogin(profileName string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := LoginProfile(profileName, cfg); err != nil {
		fmt.Printf("\nLogin failed: %v\n", err)
		os.Exit(1)
	}
}

func handleEnv(profileName string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("# Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := GenerateEnvOutputs(profileName, cfg); err != nil {
		fmt.Printf("# Error: %v\n", err)
		os.Exit(1)
	}
}

func handleConsole(profileName string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	project := ""
	account := ""

	if profileName != "" {
		// Use specified profile
		profile, ok := cfg.Profiles[profileName]
		if !ok {
			fmt.Printf("Error: Profile %q not found\n", profileName)
			os.Exit(1)
		}
		project = profile.Project
		account = profile.Account
	} else {
		// Use active profile from environment
		project = os.Getenv("CLOUDSDK_CORE_PROJECT")
		account = os.Getenv("CLOUDSDK_CORE_ACCOUNT")
		if project == "" || account == "" {
			fmt.Println("Error: No active profile in this shell, and no profile name specified.")
			fmt.Println("Usage: gcp-sso console [<profile-name>]")
			os.Exit(1)
		}
	}

	// Generate Console URL
	// Google Console supports ?project=ID and ?authuser=email
	url := fmt.Sprintf("https://console.cloud.google.com/?project=%s&authuser=%s", project, account)
	
	fmt.Printf("Opening Google Cloud Console for Project: %s, Account: %s...\n", project, account)
	fmt.Println(url)

	if err := openURL(url); err != nil {
		fmt.Printf("Failed to automatically open browser: %v\n", err)
		fmt.Println("Please open the link manually above.")
	}
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}
