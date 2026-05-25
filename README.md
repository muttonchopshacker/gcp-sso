# gcp-sso

`gcp-sso` (GCP Service Profile Switcher) is a lightweight, fast, and secure command-line utility for Google Cloud Platform (GCP) designed to provide a seamless, session-isolated profile switching experience.

It allows you to manage multiple GCP accounts, projects, and impersonated service accounts simultaneously across different terminal sessions without mutual interference.

---

## 🚀 Key Features

*   **Terminal-Level Isolation:** Switch profiles in a single shell session without affecting other terminal windows (powered by environment variable overrides instead of global `gcloud` configurations).
*   **Service Account Impersonation:** Native, effortless support for GCP Service Account impersonation.
*   **Isolated ADC Cache:** Bypasses the single global Application Default Credentials (ADC) limitation by caching ADCs per account and dynamically linking `GOOGLE_APPLICATION_CREDENTIALS`.
*   **Isolated Kubernetes (`kubectl`) Sessions:** Automatically generates and isolates GKE cluster credentials in a profile-specific `KUBECONFIG` file.
*   **Interactive Console Access:** Instantly generates a deep-link and opens the Google Cloud Console with the correct target project and user account pre-selected.
*   **Zero External Dependencies:** Built purely using the Go standard library, making it incredibly fast, portable, and easy to compile.

---

## 🛠️ Installation

### Prerequisites
*   [Go](https://golang.org/doc/install) (1.18 or later)
*   [Google Cloud CLI (gcloud)](https://cloud.google.com/sdk/docs/install)
*   [kubectl](https://kubernetes.io/docs/tasks/tools/) (optional, for GKE integration)

### Building from Source
1.  Clone the repository or copy the source files.
2.  Initialize the Go module and build:
    ```bash
    go mod init gcp-sso-cli
    go build -o gcp-sso .
    ```
3.  Move the binary to your `PATH` (e.g., `~/bin` or `/usr/local/bin`):
    ```bash
    mkdir -p ~/bin
    mv gcp-sso ~/bin/
    ```

---

## 🐚 Shell Integration

To enable session-isolated switching, you must add a wrapper function to your shell profile (e.g., `~/.bashrc`, `~/.zshrc`).

Append the following function to your shell profile:

```bash
# GCP SSO CLI shell integration
gsp() {
  if [ $# -eq 0 ]; then
    gcp-sso list
    return 0
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
```

After saving, reload your shell:
```bash
source ~/.bashrc  # or ~/.zshrc
```

---

## ⚙️ Configuration

### 1. Initialize Configuration
Run the following command to initialize the configuration directory and create a default template:
```bash
gcp-sso init
```
This creates a config file at `~/.config/gcp-sso/config.json`.

### 2. Configure Profiles via CLI (Recommended)

You can manage your profiles entirely from the command line without manually editing JSON files.

#### A. Interactive Wizard (Best for daily use)
To configure a profile step-by-step with interactive prompts and current values as defaults:
```bash
gcp-sso configure my-dev-profile
```

#### B. Declarative Flags (Best for scripts and automation)
To create or update a profile in a single command using flags:
```bash
# Simple profile
gcp-sso configure set my-dev-profile --account=user@company.com --project=my-dev-project-123

# Advanced profile with GKE and Impersonation
gcp-sso configure set my-prod-profile \
  --account=admin@company.com \
  --project=my-prod-project-456 \
  --impersonate-sa=prod-admin-sa@my-prod-project-456.iam.gserviceaccount.com \
  --gke-cluster=prod-main-cluster \
  --gke-location=us-central1
```

#### C. Delete a Profile
To remove a profile from your configuration:
```bash
gcp-sso configure delete my-dev-profile
```

### 3. Manual JSON Editing (Alternative)
If you prefer, you can still edit the `~/.config/gcp-sso/config.json` file directly. Here is the schema structure:

    ```json
    {
      "profiles": {
        "dev-developer": {
          "account": "user@company.com",
          "project": "my-dev-project-123",
          "region": "us-central1",
          "zone": "us-central1-a"
        },
        "prod-read": {
          "account": "user@company.com",
          "project": "my-prod-project-456",
          "impersonate_service_account": "prod-reader-sa@my-prod-project-456.iam.gserviceaccount.com"
        },
        "prod-admin": {
          "account": "admin-account@company.com",
          "project": "my-prod-project-456",
          "impersonate_service_account": "prod-admin-sa@my-prod-project-456.iam.gserviceaccount.com",
          "gke": {
            "cluster": "prod-main-cluster",
            "location": "us-central1"
          }
        }
      }
    }
    ```

---

## 📖 Usage

### 1. List Profiles
List all configured profiles:
```bash
gcp-sso list
```

### 2. Connect to a Profile (Subshell Mode - Recommended)
To drop directly into an isolated terminal session pre-configured for a specific profile, run:
```bash
gcp-sso shell dev-developer
```
*Or `gcp-sso connect dev-developer`*

*   **Auto-Login:** If the profile is not logged in yet (or GKE is not bootstrapped), the tool will **automatically run the login flow first** and then drop you into the shell!
*   **Visual Feedback:** The command spawns a new subshell with a custom prompt indicator `[gcp-sso: dev-developer]`.
*   **Exit:** To exit the profile and return to your original clean terminal, simply type:
    ```bash
    exit
    ```
This mode is highly recommended for running different profiles side-by-side in separate terminal tabs, `tmux` panes, or `screen` windows.

### 3. View Status
Within an active profile shell, check the current environment state:
```bash
gcp-sso status
```

### 4. Open Cloud Console
Directly open the Google Cloud Console web page pre-selected to the profile's project and user account:
```bash
gcp-sso console
```

---

## 🐚 Alternative: In-Shell Switching (Eval Mode)

If you prefer to switch profiles directly in your current shell without spawning a subshell process, you can use the shell wrapper integration.

1.  **Configure Shell Integration:** Follow the steps in the [Shell Integration](#-shell-integration) section.
2.  **Bootstrap/Login (Manually):**
    ```bash
    gcp-sso login dev-developer
    ```
3.  **Switch Profile:**
    ```bash
    gsp dev-developer
    ```
4.  **Clear Profile:**
    ```bash
    gsp off
    ```

---

## 🤝 Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

---

## 📄 License

This project is licensed under the MIT License.
