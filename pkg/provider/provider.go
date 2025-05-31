package provider

import (
	"context"
	"fmt"
	"garm-provider-harvester/pkg/utils"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/harvester/harvester/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const version = "v0.0.1"

var namespace = "garm-runners"

const (
	defaultVMGenerateName = "harv-"
	defaultVMNamespace    = "default"

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

func (h harvesterProvider) ListInstances(ctx context.Context, poolID string) ([]params.ProviderInstance, error) {
	opts := v1.ListOptions{}
	vms, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	var res []params.ProviderInstance
	for _, vm := range vms.Items {
		if vm.Labels["pool-id"] == poolID || true {
			res = append(res, params.ProviderInstance{
				ProviderID: string(vm.UID),
				Name:       vm.Name,
				OSArch:     params.OSArch(vm.Spec.Template.Spec.Architecture),
				OSType:     params.OSType(vm.Labels["harvesterhci.io/os"]),
				Status:     params.InstanceStatus(utils.StatusMap[string(vm.Status.PrintableStatus)]),
			})
		}
	}
	return res, nil
}

// CreateInstance implements executionv011.ExternalProvider.
func (h *harvesterProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.ProviderInstance, error) {

	coresStr, memory, disk, err := utils.ParseFlavor(bootstrapParams.Flavor)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	cores, err := strconv.Atoi(coresStr)
	if err != nil {
		return params.ProviderInstance{}, err
	}

	labels := map[string]string{
		"harvesterhci.io/os": "Linux",
		"pool-id":            bootstrapParams.PoolID,
	}
	for _, labelStr := range bootstrapParams.Labels {
		slog.Info("label: ", labelStr)
	}

	// create vm
	var runnerTool params.RunnerApplicationDownload
	archMapping := map[string]string{
		"x64": "amd64",
	}
	for _, tool := range bootstrapParams.Tools {
		if strings.EqualFold(*tool.OS, string(bootstrapParams.OSType)) &&
			strings.EqualFold(archMapping[*tool.Architecture], string(bootstrapParams.OSArch)) {
			runnerTool = tool
		}
	}
	cloudInitRaw, err := cloudconfig.GetCloudConfig(bootstrapParams, runnerTool, bootstrapParams.Name)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	cloudConfigSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", strings.ToLower(bootstrapParams.Name), "cloudinit"),
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}
	if len(cloudInitRaw) > utils.CloudInitNoCloudLimitSize {
		cloudConfigSecret.Data["userdata"] = []byte(cloudInitRaw)
	}
	cloudInitSource := builder.CloudInitSource{
		CloudInitType:      builder.CloudInitTypeNoCloud,
		UserDataSecretName: fmt.Sprintf("%s-%s", strings.ToLower(bootstrapParams.Name), "cloudinit"),
	}
	if err != nil {
		return params.ProviderInstance{}, err
	}
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

	// newVm := builder.NewVMBuilder("garm").Namespace(namespace).Labels(labels).
	// 	Name("garm-test").CPU(defaultVMCPUCores).Memory(defaultVMMemory).Run(true)
	opts := v1.CreateOptions{}
	var res *kubevirtv1.VirtualMachine
	res, err = h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Create(ctx, vm, opts)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	cloudConfigSecret.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: vm.APIVersion,
			Kind:       vm.Kind,
			Name:       strings.ToLower(vm.Name),
			UID:        res.UID,
		},
	}
	_, err = h.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &cloudConfigSecret, metav1.CreateOptions{})
	if err != nil {
		return params.ProviderInstance{}, err
	}

	return params.ProviderInstance{
		Name: res.Name,
	}, nil
}

// DeleteInstance implements executionv011.ExternalProvider.
func (h *harvesterProvider) DeleteInstance(ctx context.Context, instance string) error {
	err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Delete(ctx, instance, v1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetInstance implements executionv011.ExternalProvider.
func (h *harvesterProvider) GetInstance(ctx context.Context, instance string) (params.ProviderInstance, error) {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Get(ctx, instance, opts)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	return params.ProviderInstance{
		ProviderID: string(vm.UID),
		Name:       vm.Name,
		OSArch:     params.OSArch(vm.Spec.Template.Spec.Architecture),
		OSType:     params.OSType(vm.Labels["harvesterhci.io/os"]),
		Status:     params.InstanceStatus(utils.StatusMap[string(vm.Status.PrintableStatus)]),
	}, nil
}

// GetVersion implements executionv011.ExternalProvider.
func (h *harvesterProvider) GetVersion(ctx context.Context) string {
	return version
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
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Get(ctx, instance, opts)
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
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(namespace).Get(ctx, instance, opts)
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
