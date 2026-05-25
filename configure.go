package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

var reader = bufio.NewReader(os.Stdin)

// handleConfigure routes configuration subcommands.
func handleConfigure() {
	if len(os.Args) < 3 {
		// No profile name or subcommand passed. Default to wizard, but we need a profile name.
		fmt.Println("Usage:")
		fmt.Println("  gcp-sso configure <profile-name>              Launch interactive setup wizard")
		fmt.Println("  gcp-sso configure set <profile-name> [flags]   Set profile properties via flags")
		fmt.Println("  gcp-sso configure delete <profile-name>        Delete a profile")
		os.Exit(1)
	}

	subCmd := os.Args[2]

	switch subCmd {
	case "set":
		if len(os.Args) < 4 {
			fmt.Println("Error: Profile name required. Usage: gcp-sso configure set <profile-name> [flags]")
			os.Exit(1)
		}
		if err := handleConfigureSet(os.Args[3], os.Args[4:]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Error: Profile name required. Usage: gcp-sso configure delete <profile-name>")
			os.Exit(1)
		}
		if err := handleConfigureDelete(os.Args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	default:
		// Assume subCmd is the profile name, launch the wizard
		if err := handleConfigureWizard(subCmd); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// handleConfigureDelete deletes a profile.
func handleConfigureDelete(profileName string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if _, ok := cfg.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	delete(cfg.Profiles, profileName)

	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Profile %q deleted successfully.\n", profileName)
	return nil
}

// handleConfigureSet handles declarative configuration via flags.
func handleConfigureSet(profileName string, args []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Define sub-flags, use ContinueOnError for testability
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	account := fs.String("account", "", "Google Account Email")
	project := fs.String("project", "", "GCP Project ID")
	region := fs.String("region", "", "Compute Region")
	zone := fs.String("zone", "", "Compute Zone")
	impersonateSA := fs.String("impersonate-sa", "", "Service Account Email to impersonate")
	gkeCluster := fs.String("gke-cluster", "", "GKE Cluster Name")
	gkeLocation := fs.String("gke-location", "", "GKE Cluster Location (Region/Zone)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	profile, exists := cfg.Profiles[profileName]

	// If creating new profile, some flags are mandatory
	if !exists {
		if *account == "" || *project == "" {
			return fmt.Errorf("--account and --project are required for creating a new profile")
		}
		profile = Profile{}
	}

	// Apply overrides if flags are specified
	if *account != "" {
		profile.Account = *account
	}
	if *project != "" {
		profile.Project = *project
	}
	if *region != "" {
		profile.Region = *region
	}
	if *zone != "" {
		profile.Zone = *zone
	}
	if *impersonateSA != "" {
		profile.ImpersonateServiceAccount = *impersonateSA
	}

	// GKE Config
	if *gkeCluster != "" || *gkeLocation != "" {
		if profile.GKE == nil {
			profile.GKE = &GKEConfig{}
		}
		if *gkeCluster != "" {
			profile.GKE.Cluster = *gkeCluster
		}
		if *gkeLocation != "" {
			profile.GKE.Location = *gkeLocation
		}
	}

	cfg.Profiles[profileName] = profile

	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	action := "created"
	if exists {
		action = "updated"
	}
	fmt.Printf("Profile %q %s successfully.\n", profileName, action)
	return nil
}

// handleConfigureWizard launches the interactive setup wizard.
func handleConfigureWizard(profileName string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Configuring profile %q...\n\n", profileName)

	profile, exists := cfg.Profiles[profileName]
	if !exists {
		profile = Profile{}
	}

	// 1. Account
	profile.Account = promptString("Enter Google Account Email", profile.Account, true)

	// 2. Project
	profile.Project = promptString("Enter GCP Project ID", profile.Project, true)

	// 3. Region
	profile.Region = promptString("Enter Compute Region (optional)", profile.Region, false)

	// 4. Zone
	profile.Zone = promptString("Enter Compute Zone (optional)", profile.Zone, false)

	// 5. GKE Cluster
	hasGKE := profile.GKE != nil && profile.GKE.Cluster != ""
	gkeConfirm := promptConfirm("Configure GKE Cluster?", hasGKE)
	if gkeConfirm {
		if profile.GKE == nil {
			profile.GKE = &GKEConfig{}
		}
		profile.GKE.Cluster = promptString("  Enter GKE Cluster Name", profile.GKE.Cluster, true)
		profile.GKE.Location = promptString("  Enter GKE Cluster Location (Region/Zone)", profile.GKE.Location, true)
	} else {
		profile.GKE = nil
	}

	// 6. Impersonation
	hasSA := profile.ImpersonateServiceAccount != ""
	saConfirm := promptConfirm("Configure Service Account Impersonation?", hasSA)
	if saConfirm {
		profile.ImpersonateServiceAccount = promptString("  Enter Service Account Email", profile.ImpersonateServiceAccount, true)
	} else {
		profile.ImpersonateServiceAccount = ""
	}

	cfg.Profiles[profileName] = profile

	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	action := "created"
	if exists {
		action = "updated"
	}
	fmt.Printf("\nProfile %q %s successfully!\n", profileName, action)
	fmt.Printf("To login, run: gcp-sso login %s\n", profileName)
	return nil
}

// Prompt Helper Utilities

func promptString(label, defaultValue string, required bool) string {
	defaultText := "none"
	if defaultValue != "" {
		defaultText = defaultValue
	}

	for {
		fmt.Printf("%s [current: %s]: ", label, defaultText)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			if defaultValue != "" {
				return defaultValue
			}
			if !required {
				return ""
			}
			fmt.Println("Error: This field is required.")
			continue
		}
		return input
	}
}

func promptConfirm(label string, currentBool bool) bool {
	defaultText := "n"
	if currentBool {
		defaultText = "y"
	}

	for {
		fmt.Printf("%s (y/n) [current: %s]: ", label, defaultText)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" {
			return currentBool
		}

		if input == "y" || input == "yes" {
			return true
		}
		if input == "n" || input == "no" {
			return false
		}

		fmt.Println("Error: Please enter 'y' or 'n'.")
	}
}
