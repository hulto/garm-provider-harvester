package provider

import (
	"context"
	"fmt"
	"garm-provider-harvester/pkg/utils"
	"reflect"
	"strings"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/harvester/harvester/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	osTypeCost = "os-type"
	poolIdConst = "pool-id"
)

var Version = "v0.0.0-unknown"

func (h harvesterProvider) ListInstances(ctx context.Context, poolID string) ([]params.ProviderInstance, error) {
	opts := v1.ListOptions{}
	vms, err := h.HarvesterClient.KubevirtV1().VirtualMachineInstances(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	var res []params.ProviderInstance
	for _, vm := range vms.Items {
		if vm.Labels[poolIdConst] == poolID || true {
			res = append(res, utils.HarvesterVmToInstance(&vm))
		}
	}
	return res, nil
}

// CreateInstance implements executionv011.ExternalProvider.
func (h *harvesterProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.ProviderInstance, error) {

	// Get resources
	cores, memory, disk, err := utils.ParseFlavor(bootstrapParams.Flavor)
	if err != nil {
		return params.ProviderInstance{}, err
	}

	// Get labels
	labels := map[string]string{
		fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, osTypeConst): string(bootstrapParams.OSType),
		fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, poolIdConst): bootstrapParams.PoolID,
	}

	// get tool
	gitArch, err := util.ResolveToGithubArch(string(bootstrapParams.OSArch))
	if err != nil {
		return params.ProviderInstance{}, err
	}

	var runnerTool params.RunnerApplicationDownload
	for _, tool := range bootstrapParams.Tools {
		if strings.EqualFold(*tool.OS, string(bootstrapParams.OSType)) &&
			strings.EqualFold(*tool.Architecture, gitArch) {
			runnerTool = tool
		}
	}
	if runnerTool == (params.RunnerApplicationDownload{}) {
		return params.ProviderInstance{}, fmt.Errorf("no tools found for %s %s", gitArch, string(bootstrapParams.OSType))
	}

	// Cloud init setup
	userData, err := cloudconfig.GetCloudConfig(bootstrapParams, runnerTool, bootstrapParams.Name)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	var cloudConfigSecret corev1.Secret
	var cloudInitSource builder.CloudInitSource
	if len(userData) > utils.CloudInitNoCloudLimitSize {
		cloudConfigSecret = corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", strings.ToLower(bootstrapParams.Name), "cloudinit"),
				Namespace: namespace,
			},
			Data: map[string][]byte{},
		}
		cloudInitSource = builder.CloudInitSource{
			CloudInitType:      builder.CloudInitTypeNoCloud,
			UserDataSecretName: fmt.Sprintf("%s-%s", strings.ToLower(bootstrapParams.Name), "cloudinit"),
		}
		cloudConfigSecret.Data["userdata"] = []byte(userData)
	} else {
		cloudInitSource = builder.CloudInitSource{
			CloudInitType: builder.CloudInitTypeNoCloud,
			UserData:      userData,
		}
	}
	if err != nil {
		return params.ProviderInstance{}, err
	}

	// Boot Disk
	// TODO: Put image in config
	// TODO: Put longhorn in const
	storageclassname := fmt.Sprintf("longhorn-%s", "noble-server-cloudimg-amd64")
	pvcOption := &builder.PersistentVolumeClaimOption{
		ImageID:          bootstrapParams.Image,
		VolumeMode:       corev1.PersistentVolumeBlock,
		AccessMode:       corev1.ReadWriteMany,
		StorageClassName: &storageclassname,
		Annotations: map[string]string{
			"terraform-provider-harvester-auto-delete": "true",
		},
	}

	// Build VM
	vmBuilder := builder.NewVMBuilder("garm-provider").NetworkInterface("nic-0", "virtio", "", "masquerade", "").
		Namespace(namespace).Name(strings.ToLower(bootstrapParams.Name)).CPU(cores).Memory(memory).
		PVCDisk("rootdisk", builder.DiskBusVirtio, false, false, 1, disk, "", pvcOption).
		CloudInitDisk(builder.CloudInitDiskName, builder.DiskBusVirtio, false, 0, cloudInitSource).
		EvictionStrategy(true).RunStrategy(kubevirtv1.RunStrategyRerunOnFailure).Labels(labels)

	vm, err := vmBuilder.VM()
	if err != nil {
		return params.ProviderInstance{}, err
	}
	vm.Kind = kubevirtv1.VirtualMachineGroupVersionKind.Kind
	vm.APIVersion = kubevirtv1.GroupVersion.String()

	// Create VM
	opts := v1.CreateOptions{}
	var res *kubevirtv1.VirtualMachine
	res, err = h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Create(ctx, vm, opts)
	if err != nil {
		return params.ProviderInstance{}, err
	}

	// Create cloud-init secret
	if cloudConfigSecret.Data != nil {
		cloudConfigSecret.OwnerReferences = []v1.OwnerReference{
			{
				APIVersion: vm.APIVersion,
				Kind:       vm.Kind,
				Name:       strings.ToLower(vm.Name),
				UID:        res.UID,
			},
		}
		_, err = h.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &cloudConfigSecret, v1.CreateOptions{})
		if err != nil {
			return params.ProviderInstance{}, err
		}
	}

	return params.ProviderInstance{
		ProviderID: string(res.UID),
		Name:       res.Name,
		OSArch:     params.OSArch(res.Spec.Template.Spec.Architecture),
		OSType:     params.OSType(params.OSType(vm.Labels[fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, osTypeConst)])),
		Status:     params.InstanceStatus(utils.StatusMap[string(vm.Status.PrintableStatus)]),
	}, nil
}

// DeleteInstance implements executionv011.ExternalProvider.
func (h *harvesterProvider) DeleteInstance(ctx context.Context, instance string) error {
	err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Delete(ctx, strings.ToLower(instance), v1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetInstance implements executionv011.ExternalProvider.
func (h *harvesterProvider) GetInstance(ctx context.Context, instance string) (params.ProviderInstance, error) {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachineInstances(namespace).Get(ctx, instance, opts)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	return utils.HarvesterVmToInstance(vm), nil
}

// GetVersion implements executionv011.ExternalProvider.
func (h *harvesterProvider) GetVersion(ctx context.Context) string {
	return Version
}

// RemoveAllInstances implements executionv011.ExternalProvider.
func (h *harvesterProvider) RemoveAllInstances(ctx context.Context) error {
	opts := v1.ListOptions{}
	vms, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).List(ctx, opts)
	if err != nil {
		return err
	}
	for _, vm := range vms.Items {
		err = h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Delete(ctx, vm.Name, v1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// Start implements executionv011.ExternalProvider.
func (h *harvesterProvider) Start(ctx context.Context, instance string) error {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Get(ctx, strings.ToLower(instance), opts)
	if err != nil {
		return err
	}
	vmCopy := vm.DeepCopy()
	runStrategy := kubevirtv1.RunStrategyRerunOnFailure
	vmCopy.Spec.RunStrategy = &runStrategy
	if !reflect.DeepEqual(vm, vmCopy) {
		_, err = h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Update(ctx, vmCopy, v1.UpdateOptions{})
		return err
	}
	return nil
}

// Stop implements executionv011.ExternalProvider.
func (h *harvesterProvider) Stop(ctx context.Context, instance string, force bool) error {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Get(ctx, strings.ToLower(instance), opts)
	if err != nil {
		return err
	}
	vmCopy := vm.DeepCopy()
	runStrategy := kubevirtv1.RunStrategyHalted
	vmCopy.Spec.RunStrategy = &runStrategy
	if !reflect.DeepEqual(vm, vmCopy) {
		_, err = h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Update(ctx, vmCopy, v1.UpdateOptions{})
		return err
	}
	return nil
}
