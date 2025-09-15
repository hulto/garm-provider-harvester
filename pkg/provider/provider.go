package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"garm-provider-harvester/pkg/config"
	"garm-provider-harvester/pkg/utils"
	"log/slog"
	"reflect"
	"strings"

	execution "github.com/cloudbase/garm-provider-common/execution/v0.1.0"
	harvnetworkclient "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned"
	harvclient "github.com/harvester/harvester/pkg/generated/clientset/versioned"
	"github.com/mitchellh/go-homedir"
	"github.com/rancher/wrangler/pkg/kubeconfig"
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
		return nil, err
	}
	return clientcmd.RESTConfigFromKubeConfig(bytes)
}

func restConfigFromFile(kubeConfig string) (*rest.Config, error) {
	clientConfigPath, err := homedir.Expand(kubeConfig)
	if err != nil {
		return nil, err
	}

	clientConfig := kubeconfig.GetNonInteractiveClientConfig(clientConfigPath)
	return clientConfig.ClientConfig()
}

var _ execution.ExternalProvider = &HarvesterProvider{}

func NewHarvesterProvider(config config.Config, garmControllerId string) (execution.ExternalProvider, error) {
	var (
		restConfig *rest.Config
		err        error
	)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	kubeConfig := config.Credentials.KubeConfig
	if restConfig, err = restConfigFromBase64(kubeConfig); err != nil {
		if restConfig, err = restConfigFromFile(kubeConfig); err != nil {
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

type ImageMetadataAnnotations struct {
	ImageId string `json:"harvesterhci.io/imageId"`
}

type ImageMetadata struct {
	Annotations ImageMetadataAnnotations `json:"annotations"`
	Name        string                   `json:"name"`
}

type Image struct {
	Metadata ImageMetadata `json:"metadata"`
}

type ImageList struct {
	Items []Image `json:"items"`
}

func (h *HarvesterProvider) getBackingImageName(ctx context.Context, imageName string) (string, error) {
	l, err := h.KubeClient.RESTClient().Get().AbsPath("/apis/longhorn.io/v1beta2/namespaces/longhorn-system/backingimages").DoRaw(ctx)
	if err != nil {
		return "", err
	}
	imagesList := &ImageList{}
	if err := json.Unmarshal(l, &imagesList); err != nil {
		return "", err
	}
	for _, img := range imagesList.Items {
		if img.Metadata.Annotations.ImageId == imageName {
			return img.Metadata.Name, nil
		}
	}
	return "", fmt.Errorf("unable to find backing image for %s", imageName)
}

func (h *HarvesterProvider) getStorageClass(ctx context.Context, backingImage string) (string, error) {
	sc, err := h.KubeClient.StorageV1().StorageClasses().List(ctx, v1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, s := range sc.Items {
		val, ok := s.Parameters["backingImage"]
		if ok {
			if val == backingImage{
				slog.Info(fmt.Sprintf("sc: %s", s.ObjectMeta.Name))
				return s.GetObjectMeta().GetName(), nil
			}
		}
	}
	return "", fmt.Errorf("backing image %s not found", backingImage)
}


// CreateInstance implements executionv011.ExternalProvider.
func (h *HarvesterProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.ProviderInstance, error) {

	if h.GarmConfig == nil {
		return params.ProviderInstance{}, fmt.Errorf("provider config cannot be nil")
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

	// Resolve image's name `harvester-public/ubunut24` to storageclass name.
	backingImage, err := h.getBackingImageName(ctx, bootstrapParams.Image)
	if err != nil {
		return params.ProviderInstance{}, err
	}
	
	storageClass, err := h.getStorageClass(ctx, backingImage)
	if err != nil {
		return params.ProviderInstance{}, err
	}

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
	vmBuilder := builder.NewVMBuilder("garm-provider").NetworkInterface("nic-0", "virtio", "", "masquerade", "").
		Namespace(h.GarmConfig.Namespace).Name(strings.ToLower(bootstrapParams.Name)).CPU(cores).Memory(memory).
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
	res, err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Create(ctx, vm, opts)
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
		_, err = h.KubeClient.CoreV1().Secrets(h.GarmConfig.Namespace).Create(ctx, &cloudConfigSecret, v1.CreateOptions{})
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
func (h *HarvesterProvider) DeleteInstance(ctx context.Context, instance string) error {
	err := h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Delete(ctx, strings.ToLower(instance), v1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetInstance implements executionv011.ExternalProvider.
func (h *HarvesterProvider) GetInstance(ctx context.Context, instance string) (params.ProviderInstance, error) {
	opts := v1.GetOptions{}
	vm, err := h.HarvesterClient.KubevirtV1().VirtualMachineInstances(h.GarmConfig.Namespace).Get(ctx, instance, opts)
	if err != nil {
		return params.ProviderInstance{}, err
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
		return err
	}
	for _, vm := range vms.Items {
		err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Delete(ctx, vm.Name, v1.DeleteOptions{})
		if err != nil {
			return err
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
	runStrategy := kubevirtv1.RunStrategyRerunOnFailure
	vmCopy.Spec.RunStrategy = &runStrategy
	if !reflect.DeepEqual(vm, vmCopy) {
		_, err = h.HarvesterClient.KubevirtV1().VirtualMachines(h.GarmConfig.Namespace).Update(ctx, vmCopy, v1.UpdateOptions{})
		return err
	}
	return nil
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
	return nil
}
