package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cloudbase/garm-provider-common/params"
	"github.com/google/uuid"
	"github.com/harvester/harvester/pkg/builder"
	"golang.org/x/exp/slog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GarmCommand string

const (
	CreateInstance     GarmCommand = "CreateInstance"
	DeleteInstance     GarmCommand = "DeleteInstance"
	GetInstance        GarmCommand = "GetInstance"
	ListInstances      GarmCommand = "ListInstances"
	RemoveAllInstances GarmCommand = "RemoveAllInstances"
	Stop               GarmCommand = "Stop"
	Start              GarmCommand = "Start"
)

func NewGarmCommand(s string) (GarmCommand, error) {
	switch s {
	case string(CreateInstance):
		return CreateInstance, nil
	case string(DeleteInstance):
		return DeleteInstance, nil
	case string(GetInstance):
		return GetInstance, nil
	case string(ListInstances):
		return ListInstances, nil
	case string(RemoveAllInstances):
		return RemoveAllInstances, nil
	case string(Stop):
		return Stop, nil
	case string(Start):
		return Start, nil
	default:
		return "", fmt.Errorf("unknown GarmCommand string: %s", s)
	}
}

func ParseArgs() (garmCommand GarmCommand, garmProviderConfigFile string, garmControllerId uuid.UUID, garmPoolId *uuid.UUID, garmInstanceId *uuid.UUID) {
	garmCommandStr, exists := os.LookupEnv("GARM_COMMAND")
	if !exists {
		slog.Error("[ERROR] GARM_COMMAND variable is required")
		os.Exit(1)
	}
	garmCommand, err := NewGarmCommand(garmCommandStr)
	if err != nil {
		slog.Error(fmt.Sprintf("[ERROR] Failed to parse %s must be: CreateInstance, DeleteInstance, GetInstance, ListInstances, RemoveAllInstances, Stop, or Start. %s\n", garmCommandStr, err))
		os.Exit(1)
	}

	garmProviderConfigFile, exists = os.LookupEnv("GARM_PROVIDER_CONFIG_FILE")
	if !exists {
		slog.Error("[ERROR] GARM_PROVIDER_CONFIG_FILE variable is required")
		os.Exit(1)
	}
	_, err = os.Stat(garmProviderConfigFile)
	if err != nil {
		slog.Error(fmt.Sprintf("[ERROR] File path %s not found: %t\n", garmProviderConfigFile, exists))
	}

	garmControllerIdStr, exists := os.LookupEnv("GARM_CONTROLLER_ID")
	if !exists {
		slog.Error("[ERROR] GARM_CONTROLLER_ID variable is required")
		os.Exit(1)
	}
	garmControllerId = uuid.MustParse(garmControllerIdStr)

	garmPoolId = nil
	garmPoolIdStr, exists := os.LookupEnv("GARM_POOL_ID")
	if exists {
		u := uuid.MustParse(garmPoolIdStr)
		garmPoolId = &u
		slog.Error("GARM_POOL_ID:", garmPoolId)
	}

	garmInstanceId = nil
	garmInstanceIdStr, exists := os.LookupEnv("GARM_INSTANCE_ID")
	if exists {
		u := uuid.MustParse(garmInstanceIdStr)
		garmPoolId = &u
		slog.Error("GARM_INSTANCE_ID:", garmPoolId)
	}

	return garmCommand, garmProviderConfigFile, garmControllerId, garmPoolId, garmInstanceId
}

var StatusMap = map[string]string{
	"ACTIVE":   "Running",
	"SHUTOFF":  "stopped",
	"BUILD":    "pending_create",
	"ERROR":    "error",
	"DELETING": "pending_delete",
}

var flavorMap = map[string][]string{
	"small":  []string{"0.5", "256Mi", "10Gi"},
	"medium": []string{"1", "2Gi", "12Gi"},
	"large":  []string{"4", "8Gi", "24Gi"},
	"xlarge": []string{"8", "16Gi", "32Gi"},
}

// Accept either a standard size or parse a custom
// custom-4c-256Mi-10Gi
func ParseFlavor(flavor string) (cpu_count string, memory string, disksize string, err error) {
	val, ok := flavorMap[flavor]
	// If the key exists
	if ok {
		return val[0], val[1], val[2], nil
	}

	parts := strings.Split(flavor, "-")
	if parts[0] != "custom" {
		return "", "", "", fmt.Errorf("unkwon flavor %s", flavor)
	}

	if !strings.HasSuffix(parts[1], "c") {
		return "", "", "", fmt.Errorf("unkwon core count format %s", parts[1])
	}

	coreCount := strings.TrimSuffix(parts[1], "c")
	_, err = strconv.ParseFloat(coreCount, 64)
	if err != nil {
		return "", "", "", err
	}

	if !strings.HasSuffix(parts[2], "Mi") || !strings.HasSuffix(parts[2], "Gi") {
		return "", "", "", fmt.Errorf("unkwon memory format %s", parts[2])
	}

	if !strings.HasSuffix(parts[3], "Mi") || !strings.HasSuffix(parts[3], "Gi") {
		return "", "", "", fmt.Errorf("unkwon disk format %s", parts[2])
	}

	return strings.TrimSuffix(parts[1], "c"), parts[2], parts[3], nil
}

const (
	userDataHeader = `#cloud-config
`
	userDataAddQemuGuestAgent = `
package_update: true
packages:
- qemu-guest-agent
runcmd:
- [systemctl, enable, --now, qemu-guest-agent]`
	userDataPasswordTemplate = `
user: %s
password: %s
chpasswd: { expire: False }
ssh_pwauth: True`

	userDataSSHKeyTemplate = `
ssh_authorized_keys:
- >-
  %s`
	userDataAddDockerGroupSSHKeyTemplate = `
groups:
- docker
users:
- name: %s
  sudo: ALL=(ALL) NOPASSWD:ALL
  groups: sudo, docker
  shell: /bin/bash
  ssh_authorized_keys:
  - >-
    %s`
	cloudInitNoCloudLimitSize = 2048
)

func BuildCloudInit(namespace string, bootstrapParams params.BootstrapInstance) (*builder.CloudInitSource, *corev1.Secret, error) {
	cloudInitSource := &builder.CloudInitSource{
		CloudInitType: builder.CloudInitTypeNoCloud,
	}
	userData, networkData, err := MergeCloudInit(namespace, bootstrapParams)
	if err != nil {
		return nil, nil, err
	}
	cloudConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", bootstrapParams.Name, "cloudinit"),
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}

	if userData != "" {
		if len(userData) > cloudInitNoCloudLimitSize {
			cloudConfigSecret.Data["userdata"] = []byte(userData)
			cloudInitSource.UserDataSecretName = cloudConfigSecret.Name
		} else {
			cloudInitSource.UserData = userData
		}
	}
	if networkData != "" {
		if len(userData) > cloudInitNoCloudLimitSize {
			cloudConfigSecret.Data["networkdata"] = []byte(networkData)
			cloudInitSource.NetworkDataSecretName = cloudConfigSecret.Name
		} else {
			cloudInitSource.NetworkData = networkData
		}
	}
	return cloudInitSource, cloudConfigSecret, nil
}

func MergeCloudInit(namespace string, bootstrapParams params.BootstrapInstance) (string, string, error) {
	var (
		userData    string
		networkData string
	)
	// userData
	userData += userDataAddQemuGuestAgent
	for _, sshkey := range bootstrapParams.SSHKeys {
		userData += fmt.Sprintf(userDataSSHKeyTemplate, sshkey)
	}
	userData = userDataHeader + userData
	return userData, networkData, nil
}
