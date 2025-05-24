package cloudinit

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/cloudbase/garm-provider-common/params"
	"github.com/google/uuid"
)

const (
	defaultSetupUser         = "runner"
	defaultRunnerInstallPath = "/opt/actions-runner"
)

const BaseLinuxCloudInitScript = `#!/bin/bash
set -euxo pipefail

export RUNNER_NAME="{{ .Name }}"
export GITHUB_RUNNER_REGISTRATION_TOKEN="{{ .InstanceToken }}"
export CALLBACK_URL="{{ .CallbackURL }}"
export RUNNER_DOWNLOAD_URL="{{ .DownloadURL }}"
export RUNNER_LABELS="{{ range $index, $label := .Labels }}{{ if $index }},{{ end }}{{ $label }}{{ end }}"
export RUNNER_TEMP_DIR="/tmp/runner-{{ .RunnerTempID }}"
export RUNNER_INSTALL_PATH="{{ .RunnerInstallPath }}"
export SETUP_USER="{{ .SetupUser }}"
export RUNNER_GROUP="${SETUP_USER}"

if ! id -u "${SETUP_USER}"; then
    sudo useradd --create-home --shell /bin/bash "${SETUP_USER}"
    sudo usermod -aG sudo "${SETUP_USER}"
    echo "${SETUP_USER} ALL=(ALL) NOPASSWD:ALL" | sudo tee "/etc/sudoers.d/90-${SETUP_USER}"
fi

if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    sudo apt-get update -y
    sudo apt-get install -y apt-transport-https ca-certificates curl software-properties-common gnupg
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update -y
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io
    sudo usermod -aG docker "${SETUP_USER}"
    echo "Docker installed."
else
    echo "Docker already installed."
fi

sudo mkdir -p "${RUNNER_INSTALL_PATH}" "${RUNNER_TEMP_DIR}"
sudo chown -R "${SETUP_USER}:${RUNNER_GROUP}" "${RUNNER_INSTALL_PATH}" "${RUNNER_TEMP_DIR}"

cd "${RUNNER_TEMP_DIR}"

echo "Downloading runner from ${RUNNER_DOWNLOAD_URL}..."
sudo -u "${SETUP_USER}" -E curl -L -o runner.tar.gz "${RUNNER_DOWNLOAD_URL}"
sudo -u "${SETUP_USER}" -E tar xzf ./runner.tar.gz -C "${RUNNER_INSTALL_PATH}"
sudo -u "${SETUP_USER}" -E rm -f ./runner.tar.gz

cd "${RUNNER_INSTALL_PATH}"
echo "Configuring runner..."
sudo -u "${SETUP_USER}" -E ./config.sh --unattended \
    --name "${RUNNER_NAME}" \
    --url https://github.com/{{ .RepoOwner }}/{{ .RepoName }} \
    --token "${GITHUB_RUNNER_REGISTRATION_TOKEN}" \
    --labels "${RUNNER_LABELS}" \
    {{ if .RunnerGroup }}--runnergroup "{{ .RunnerGroup }}"{{ end }} \
    --work "_work" \
    --replace \
    --ephemeral

echo "Setting up runner service..."
sudo ./svc.sh install "${SETUP_USER}"
sudo ./svc.sh start

if [ -n "${CALLBACK_URL}" ] && [ -n "${GITHUB_RUNNER_REGISTRATION_TOKEN}" ]; then
    echo "Sending callback to GARM..."
    CALLBACK_DATA="{\"status\": \"success\", \"runner_name\": \"${RUNNER_NAME}\", \"message\": \"Runner configured successfully\"}"
    MAX_ATTEMPTS=5
    RETRY_DELAY=10
    for i in $(seq 1 $MAX_ATTEMPTS); do
        HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
            -X POST \
            -H "Content-Type: application/json" \
            -H "X-Garm-Token: ${GITHUB_RUNNER_REGISTRATION_TOKEN}" \
            -d "${CALLBACK_DATA}" \
            "${CALLBACK_URL}")
        
        if [ "${HTTP_STATUS}" -eq 200 ] || [ "${HTTP_STATUS}" -eq 202 ]; then
            echo "Callback successful (HTTP ${HTTP_STATUS})."
            break
        else
            echo "Callback attempt ${i} failed (HTTP ${HTTP_STATUS}). Retrying in ${RETRY_DELAY}s..."
            sleep ${RETRY_DELAY}
        fi
        if [ ${i} -eq ${MAX_ATTEMPTS} ]; then
            echo "Max callback attempts reached. Failed to send status to GARM."
        fi
    done
else
    echo "Callback URL or Instance Token not set, skipping callback."
fi

echo "Cloud-init script finished."
`

type CloudInitTemplateData struct {
	Name              string
	InstanceToken     string
	CallbackURL       string
	Labels            []string
	DownloadURL       string
	RepoOwner         string
	RepoName          string
	RunnerGroup       string
	RunnerInstallPath string
	SetupUser         string
	RunnerTempID      string
}

func GenerateCloudInit(bootstrapParams params.BootstrapInstance) (string, error) {
	var scriptTemplate string

	switch bootstrapParams.OSType {
	case params.Linux:
		scriptTemplate = BaseLinuxCloudInitScript
	default:
		return "", fmt.Errorf("unsupported OS type for cloud-init: %s", bootstrapParams.OSType)
	}

	repoURL := strings.TrimPrefix(bootstrapParams.RepoURL, "https://")
	parts := strings.SplitN(repoURL, "/", 3)
	var repoOwner, repoName string
	if len(parts) >= 2 {
		repoOwner = parts[1]
		if len(parts) == 3 {
			repoName = parts[2]
		}
	} else {
		return "", fmt.Errorf("invalid repository URL format: %s", bootstrapParams.RepoURL)
	}

	runnerTempID := uuid.New().String()
	var downloadURL string
	// Simplified tool selection: pick the first available URL.
	// This bypasses the problematic tool.OSType/tool.OSArch field access.
	if len(bootstrapParams.Tools) > 0 && bootstrapParams.Tools[0].DownloadURL != nil {
		downloadURL = *bootstrapParams.Tools[0].DownloadURL
	} else {
		return "", fmt.Errorf("no runner download URL found in bootstrap params")
	}

	var runnerGroup string // Will be empty for v0.1.4 of params

	templateData := CloudInitTemplateData{
		Name:              bootstrapParams.Name,
		InstanceToken:     bootstrapParams.InstanceToken,
		CallbackURL:       bootstrapParams.CallbackURL,
		Labels:            bootstrapParams.Labels,
		DownloadURL:       downloadURL,
		RepoOwner:         repoOwner,
		RepoName:          repoName,
		RunnerGroup:       runnerGroup,
		RunnerInstallPath: defaultRunnerInstallPath,
		SetupUser:         defaultSetupUser,
		RunnerTempID:      runnerTempID,
	}

	tmpl, err := template.New("cloud-init").Parse(scriptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse cloud-init template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute cloud-init template: %w", err)
	}

	return buf.String(), nil
}
