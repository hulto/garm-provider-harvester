package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudbase/garm-provider-common/params"
	"github.com/harvester/harvester/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

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
	CloudInitNoCloudLimitSize = 2048
	defaultVMGenerateName     = "garm-"
	defaultVMNamespace        = "default"

	defaultVMCPUCores = 1
	defaultVMMemory   = "256Mi"

	HarvesterAPIGroup                                     = "harvesterhci.io"
	LabelAnnotationPrefixHarvester                        = HarvesterAPIGroup + "/"
	LabelKeyVirtualMachineCreator                         = LabelAnnotationPrefixHarvester + "creator"
	LabelKeyVirtualMachineName                            = LabelAnnotationPrefixHarvester + "vmName"
	AnnotationKeyVirtualMachineSSHNames                   = LabelAnnotationPrefixHarvester + "sshNames"
	AnnotationKeyVirtualMachineWaitForLeaseInterfaceNames = LabelAnnotationPrefixHarvester + "waitForLeaseInterfaceNames"
	AnnotationKeyVirtualMachineDiskNames                  = LabelAnnotationPrefixHarvester + "diskNames"
	AnnotationKeyImageID                                  = LabelAnnotationPrefixHarvester + "imageId"

	AnnotationPrefixCattleField = "field.cattle.io/"
	LabelPrefixHarvesterTag     = "tag.harvesterhci.io/"
	AnnotationKeyDescription    = AnnotationPrefixCattleField + "description"
)

var StatusMap = map[string]string{
	"ACTIVE":   "Running",
	"SHUTOFF":  "stopped",
	"BUILD":    "pending_create",
	"ERROR":    "error",
	"DELETING": "pending_delete",
}

var flavorMap = map[string][]string{
	"small":  []string{"1", "256Mi", "10Gi"},
	"medium": []string{"1", "2Gi", "12Gi"},
	"large":  []string{"4", "8Gi", "24Gi"},
	"xlarge": []string{"8", "16Gi", "32Gi"},
}

func HarvesterVmToInstance(vm *kubevirtv1.VirtualMachineInstance) params.ProviderInstance {
	addresses := []params.Address{}
	for _, net := range vm.Status.Interfaces {
		for _, ip := range net.IPs {
			addresses = append(addresses, params.Address{
				Address: ip,
				Type:    params.PrivateAddress,
			})
		}
	}

	return params.ProviderInstance{
		ProviderID: string(vm.UID),
		Name:       vm.Name,
		OSArch:     params.OSArch(vm.Spec.Architecture),
		OSType:     params.OSType(vm.Labels[fmt.Sprintf("%s/%s", HarvesterAPIGroup, "os-type")]),
		Status:     params.InstanceStatus(StatusMap[string(vm.Status.Phase)]),
		Addresses:  addresses,
	}
}

// Accept either a standard size or parse a custom
// custom-4c-256Mi-10Gi
func ParseFlavor(flavor string) (cpu_count int, memory string, disksize string, err error) {
	val, ok := flavorMap[flavor]
	// If the key exists
	if ok {
		cores, err := strconv.Atoi(val[0])
		if err != nil {
			return 0, "", "", err
		}

		return cores, val[1], val[2], nil
	}

	parts := strings.Split(flavor, "-")
	if parts[0] != "custom" {
		return 0, "", "", fmt.Errorf("unkwon flavor %s", flavor)
	}

	if !strings.HasSuffix(parts[1], "c") {
		return 0, "", "", fmt.Errorf("unkwon core count format %s", parts[1])
	}

	coreCount := strings.TrimSuffix(parts[1], "c")
	cores, err := strconv.Atoi(coreCount)
	if err != nil {
		return 0, "", "", err
	}

	if !strings.HasSuffix(parts[2], "Mi") && !strings.HasSuffix(parts[2], "Gi") {
		return 0, "", "", fmt.Errorf("unkwon memory format %s", parts[2])
	}

	if !strings.HasSuffix(parts[3], "Mi") && !strings.HasSuffix(parts[3], "Gi") {
		return 0, "", "", fmt.Errorf("unkwon disk format %s", parts[2])
	}

	return cores, parts[2], parts[3], nil
}

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
		if len(userData) > CloudInitNoCloudLimitSize {
			cloudConfigSecret.Data["userdata"] = []byte(userData)
			cloudInitSource.UserDataSecretName = cloudConfigSecret.Name
		} else {
			cloudInitSource.UserData = userData
		}
	}
	if networkData != "" {
		if len(userData) > CloudInitNoCloudLimitSize {
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
