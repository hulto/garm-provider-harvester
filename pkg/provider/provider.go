package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"garm-provider-harvester/pkg/config"
	"garm-provider-harvester/pkg/utils"
	"log/slog"
	"os"
	"reflect"
	"strings"

	execution "github.com/cloudbase/garm-provider-common/execution/v0.1.0"
	harvnetworkclient "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned"
	harvclient "github.com/harvester/harvester/pkg/generated/clientset/versioned"
	"github.com/mitchellh/go-homedir"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	storageclient "k8s.io/client-go/kubernetes/typed/storage/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/harvester/harvester/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	osTypeConst = "os-type"
	poolIdConst = "pool-id"
	controllerIdConst = "controller-id"
)

type HarvesterProvider struct {
	GarmConfig                *config.Config
	RestConfig                *rest.Config
	KubeVirtSubresourceClient *rest.RESTClient
	KubeClient                *kubernetes.Clientset
	StorageClassClient        *storageclient.StorageV1Client
	HarvesterClient           *harvclient.Clientset
	HarvesterNetworkClient    *harvnetworkclient.Clientset
	ControllerID              string
}

var Version = "v0.0.0-unknown"

func restConfigFromBase64(kubeConfigBase64 string) (*rest.Config, error) {
	bytes, err := base64.StdEncoding.DecodeString(kubeConfigBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 string with error: %s", err.Error())
	}
	return clientcmd.RESTConfigFromKubeConfig(bytes)
}

func restConfigFromFile(kubeConfig string) (*rest.Config, error) {
	clientConfigPath, err := homedir.Expand(kubeConfig)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(clientConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig %.80s with error: %s", kubeConfig, err.Error())
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from %.80s with error: %s", kubeConfig, err.Error())
	}

	return clientConfig.ClientConfig()
}

var _ execution.ExternalProvider = &HarvesterProvider{}

func NewHarvesterProvider(config config.Config, garmControllerId string) (execution.ExternalProvider, error) {
	var (
		restConfig *rest.Config
		err        error
	)
	slog.Debug(fmt.Sprintf("Creating new harvester provider: %s", garmControllerId))
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	kubeConfig := config.Credentials.KubeConfig
	if restConfig, err = restConfigFromBase64(kubeConfig); err != nil {
		slog.Debug("Not base64")
		if restConfig, err = restConfigFromFile(kubeConfig); err != nil {
			slog.Debug("Not file")
			return nil, err
		}
	}

	copyConfig := rest.CopyConfig(restConfig)
	copyConfig.GroupVersion = &kubeschema.GroupVersion{Group: "subresources.kubevirt.io", Version: "v1"}
	copyConfig.APIPath = "/apis"
	copyConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	restClient, err := rest.RESTClientFor(copyConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	storageClassClient, err := storageclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	harvClient, err := harvclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	harvNetworkClient, err := harvnetworkclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return &HarvesterProvider{
		GarmConfig:                &config,
		RestConfig:                restConfig,
		KubeVirtSubresourceClient: restClient,
		KubeClient:                kubeClient,
		StorageClassClient:        storageClassClient,
		HarvesterClient:           harvClient,
		HarvesterNetworkClient:    harvNetworkClient,
		ControllerID:              garmControllerId,
	}, nil
}

func (h HarvesterProvider) ListInstances(ctx context.Context, poolID string) ([]params.ProviderInstance, error) {
	opts := v1.ListOptions{}
	vms, err := h.HarvesterClient.KubevirtV1().VirtualMachineInstances(h.GarmConfig.Namespace).List(ctx, opts)
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


type ImageMetadataStatus struct {
	StorageClassName string				`json:"storageClassName"`
}

type ItemMetadata struct {
	Name string							`json:"name"`
}

type Item struct {
	Status ImageMetadataStatus			`json:"status"`
	Metadata ItemMetadata 				`json:"metadata"`
}

type ImageList struct {
	Items []Item 						`json:"items"`
}

// /kubectl get virtualmachineimages.harvesterhci.io -n harvester-public -o jsonpath='{.items[?(@.metadata.labels.harvesterhci\.io\/imageDisplayName == "ubuntu-server-noble-24.04")].status.storageClassName}' 
func (h *HarvesterProvider) getStorageClass(ctx context.Context, imageName string) (string, error) {
	ns := strings.Split(imageName, "/")[0]
	name := strings.Join(strings.Split(imageName, "/")[1:], "/")
	l, err := h.KubeClient.RESTClient().Get().AbsPath(fmt.Sprintf("/apis/harvesterhci.io/v1beta1/namespaces/%s/virtualmachineimages", ns)).DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query storage class for backingimage %s: %s", name, err)
	}
	imagesList := &ImageList{}
	if err := json.Unmarshal(l, &imagesList); err != nil {
		return "", fmt.Errorf("failed to unmarshal imagelist JSON for %s: %s", imageName, err)
	}
	for _, img := range imagesList.Items {
		if img.Metadata.Name == name {
			return img.Status.StorageClassName, nil
		}
	}
	return "", fmt.Errorf("backing image %s not found", imageName)
}


// CreateInstance implements executionv011.ExternalProvider.
func (h *HarvesterProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.ProviderInstance, error) {
	slog.Info(fmt.Sprintf("Create instance: %s", bootstrapParams.Name))
	if h.GarmConfig == nil {
		return params.ProviderInstance{}, fmt.Errorf("provider config cannot be nil")
	}

	extraSpec := &config.HarvesterExtraSpec{}
	if err := json.Unmarshal(bootstrapParams.ExtraSpecs, &extraSpec); err != nil {
		return params.ProviderInstance{}, fmt.Errorf("failed to unmarshal extra spec JSON for %s: %s", bootstrapParams.Name, err)
	}
	err := extraSpec.Validate()
	if err != nil {
		return params.ProviderInstance{}, fmt.Errorf("failed to validate extra spec for %s: %s", bootstrapParams.Name, err)
	}

	// Set defaults
	var networkName = "mgmt"
	if extraSpec.NetworkName != "" {
		networkName = extraSpec.NetworkName
	}
	var networkAdapterType = "virtio"
	if extraSpec.NetworkAdapterType != "" {
		networkAdapterType = extraSpec.NetworkAdapterType
	}
	var networkType = "masquerade"
	if extraSpec.NetworkType != "" {
		networkType = extraSpec.NetworkType
	}
	var diskConnectorType = "virtio"
	if extraSpec.DiskConnectorType != "" {
		diskConnectorType = extraSpec.DiskConnectorType
	}

	// Get resources
	cores, memory, disk, err := utils.ParseFlavor(bootstrapParams.Flavor)
	if err != nil {
		return params.ProviderInstance{}, err
	}

	// Get labels
	labels := map[string]string{
		fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, osTypeConst): string(bootstrapParams.OSType),
		fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, poolIdConst): bootstrapParams.PoolID,
		fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, controllerIdConst): h.ControllerID,
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
	slog.Info(fmt.Sprintf("%s: got tools", bootstrapParams.Name))

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
				Namespace: h.GarmConfig.Namespace,
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
	slog.Info(fmt.Sprintf("%s: cloud-init ready", bootstrapParams.Name))

	storageClass, err := h.getStorageClass(ctx, bootstrapParams.Image)
	if err != nil {
		slog.Info(fmt.Sprintf("%s: failed to find storage class %s %s: %s", bootstrapParams.Name, bootstrapParams.Image, storageClass, err.Error()))
		return params.ProviderInstance{}, err
	}
	slog.Info(fmt.Sprintf("%s: boot image resolved", bootstrapParams.Name))


	// Boot Disk
	pvcOption := &builder.PersistentVolumeClaimOption{
		ImageID:          bootstrapParams.Image,
		VolumeMode:       corev1.PersistentVolumeBlock,
		AccessMode:       corev1.ReadWriteMany,
		StorageClassName: &storageClass,
		Annotations: map[string]string{
			"terraform-provider-harvester-auto-delete": "true",
		},
	}

	// Build VM
	vmBuilder := builder.NewVMBuilder("garm-provider").NetworkInterface("nic-0", networkAdapterType, "", networkType, networkName).
		Namespace(h.GarmConfig.Namespace).Name(strings.ToLower(bootstrapParams.Name)).CPU(cores).Memory(memory).
		PVCDisk("rootdisk", diskConnectorType, false, false, 1, disk, "", pvcOption).
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
	res, err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Create(ctx, vm, opts)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	slog.Info(fmt.Sprintf("%s: instance created", bootstrapParams.Name))


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
		_, err = h.KubeClient.CoreV1().Secrets(h.GarmConfig.Namespace).Create(ctx, &cloudConfigSecret, v1.CreateOptions{})
		if err != nil {
			return params.ProviderInstance{}, err
		}
	}
	slog.Info(fmt.Sprintf("%s: sucess exiting", bootstrapParams.Name))

	// params.InstanceStatus(utils.StatusMap[string(vm.Status.PrintableStatus)])
	return params.ProviderInstance{
		ProviderID: strings.ToLower(bootstrapParams.Name),
		Name:       res.Name,
		OSArch:     params.OSArch(res.Spec.Template.Spec.Architecture),
		OSType:     params.OSType(params.OSType(vm.Labels[fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, osTypeConst)])),
		Status:     "running",
	}, nil
}


func  (h *HarvesterProvider) vpcsToRemove(ctx context.Context, vm *kubevirtv1.VirtualMachine) ([]string, error) {
	deleteConfigs := make(map[string]bool)
	removedPVCs := make([]string, 0, len(vm.Spec.Template.Spec.Volumes))
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		if autoDelete, ok := deleteConfigs[volume.Name]; ok && !autoDelete {
			continue
		}

		// h.KubeClient.StorageV1().VolumeAttachments().Delete(ctx, volume.Att)

		removedPVCs = append(removedPVCs, volume.PersistentVolumeClaim.ClaimName)
	}
	return removedPVCs, nil
}
// DeleteInstance implements executionv011.ExternalProvider.
func (h *HarvesterProvider) DeleteInstance(ctx context.Context, instance string) error {
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Get(ctx, strings.ToLower(instance), v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("instance %s not found: %s", strings.ToLower(instance), err.Error())
		}
		return err
	}

	val, ok := vm.Labels[fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, controllerIdConst)]
	if !ok || val != h.ControllerID {
		return fmt.Errorf("found instance %s but doesn't have label %s/%s=%s", strings.ToLower(instance), utils.HarvesterAPIGroup, controllerIdConst, h.ControllerID)
	}

	pvcsToRemove, err := h.vpcsToRemove(ctx, vm)
	if err != nil {
			return fmt.Errorf("failed to find vpcs for %s: %s", strings.ToLower(instance), err.Error())
	}


	propagationPolicy := v1.DeletePropagationForeground
	deleteOptions := v1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Delete(ctx, strings.ToLower(instance), deleteOptions)
	if err != nil {
		if apierrors.IsNotFound(err) {
			slog.Info(fmt.Sprintf("instance %s not found",instance))
			return nil
		}
		return err
	}

	for _, pvc := range pvcsToRemove {
		propagationPolicy := v1.DeletePropagationForeground
		deleteOptions := v1.DeleteOptions{PropagationPolicy: &propagationPolicy}
		err := h.KubeClient.CoreV1().PersistentVolumeClaims(h.GarmConfig.Namespace).Delete(ctx, pvc, deleteOptions)
		if err != nil {
			return fmt.Errorf("failed to remove %s pvc %s: %s", instance, pvc, err.Error())
		}
	}

	return nil
}

// GetInstance implements executionv011.ExternalProvider.
func (h *HarvesterProvider) GetInstance(ctx context.Context, instance string) (params.ProviderInstance, error) {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachineInstances(h.GarmConfig.Namespace).Get(ctx, strings.ToLower(instance), opts)
	if err != nil {
		return params.ProviderInstance{}, fmt.Errorf("failed to get instance %s: %s", strings.ToLower(instance), err.Error())
	}
	return utils.HarvesterVmToInstance(vm), nil
}

// GetVersion implements executionv011.ExternalProvider.
func (h *HarvesterProvider) GetVersion(ctx context.Context) string {
	return Version
}

// RemoveAllInstances implements executionv011.ExternalProvider.
func (h *HarvesterProvider) RemoveAllInstances(ctx context.Context) error {
	opts := v1.ListOptions{}
	vms, err := h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).List(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to get VM list for NS %s: %s", h.GarmConfig.Namespace, err.Error())
	}
	for _, vm := range vms.Items {
		val, ok := vm.Labels[fmt.Sprintf("%s/%s", utils.HarvesterAPIGroup, controllerIdConst)]
		if !ok || val != h.ControllerID {
			slog.Debug(fmt.Sprintf("found instance %s but doesn't have label %s/%s=%s", strings.ToLower(vm.Name), utils.HarvesterAPIGroup, controllerIdConst, h.ControllerID))
			continue
		}

		pvcsToRemove, err := h.vpcsToRemove(ctx, &vm)
		if err != nil {
			return fmt.Errorf("failed to find vpcs for: %s", err.Error())
		}

		err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Delete(ctx, vm.Name, v1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete virtual machine %s: %s", vm.Name, err.Error())
		}

		for _, pvc := range pvcsToRemove {
			propagationPolicy := v1.DeletePropagationForeground
			deleteOptions := v1.DeleteOptions{PropagationPolicy: &propagationPolicy}
			err := h.KubeClient.CoreV1().PersistentVolumeClaims(h.GarmConfig.Namespace).Delete(ctx, pvc, deleteOptions)
			if err != nil {
				return fmt.Errorf("failed to remove %s pvc %s: %s", vm.Name, pvc, err.Error())
			}
		}
	}
	return nil
}

// Start implements executionv011.ExternalProvider.
func (h *HarvesterProvider) Start(ctx context.Context, instance string) error {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Get(ctx, strings.ToLower(instance), opts)
	if err != nil {
		return err
	}
	vmCopy := vm.DeepCopy()
	runStrategy := kubevirtv1.RunStrategyAlways
	vmCopy.Spec.RunStrategy = &runStrategy
	if !reflect.DeepEqual(vm, vmCopy) {
		_, err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Update(ctx, vmCopy, v1.UpdateOptions{})
		return err
	}
	return fmt.Errorf("VM runstrategy is already set to %s", runStrategy)
}

// Stop implements executionv011.ExternalProvider.
func (h *HarvesterProvider) Stop(ctx context.Context, instance string, force bool) error {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Get(ctx, strings.ToLower(instance), opts)
	if err != nil {
		return err
	}
	vmCopy := vm.DeepCopy()
	runStrategy := kubevirtv1.RunStrategyHalted
	vmCopy.Spec.RunStrategy = &runStrategy
	if !reflect.DeepEqual(vm, vmCopy) {
		_, err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Update(ctx, vmCopy, v1.UpdateOptions{})
		return err
	}
	return fmt.Errorf("VM runstrategy is already set to %s", runStrategy)
}
