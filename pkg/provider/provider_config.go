package provider

import (
	"context"
	"encoding/base64"

	harvclient "github.com/harvester/harvester/pkg/generated/clientset/versioned"

	execution "github.com/cloudbase/garm-provider-common/execution/v0.1.1"
	harvnetworkclient "github.com/harvester/harvester-network-controller/pkg/generated/clientset/versioned"
	"github.com/mitchellh/go-homedir"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	storageclient "k8s.io/client-go/kubernetes/typed/storage/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type harvesterProvider struct {
	RestConfig                *rest.Config
	KubeVirtSubresourceClient *rest.RESTClient
	KubeClient                *kubernetes.Clientset
	StorageClassClient        *storageclient.StorageV1Client
	HarvesterClient           *harvclient.Clientset
	HarvesterNetworkClient    *harvnetworkclient.Clientset
	ControllerID              string
}

type Credentials struct {
	KubeConfig string `toml:"kubeconfig"`
}

// TODO: Add disk size to override VM image size disk.
type Config struct {
	Namespace string `toml:"namespace"`
	Credentials Credentials `toml:"credentials"`
	StorageClassName string `toml:"storageclassname"`
}


// GetConfigJSONSchema implements executionv011.ExternalProvider.
func (c *harvesterProvider) GetConfigJSONSchema(ctx context.Context) (string, error) {
	return "", nil
}

// GetExtraSpecsJSONSchema implements executionv011.ExternalProvider.
func (c *harvesterProvider) GetExtraSpecsJSONSchema(ctx context.Context) (string, error) {
	return "", nil
}

// GetSupportedInterfaceVersions implements executionv011.ExternalProvider.
func (c *harvesterProvider) GetSupportedInterfaceVersions(ctx context.Context) []string {
	return []string{"v0.1.0", "v0.1.1"}
}

// ValidatePoolInfo implements executionv011.ExternalProvider.
func (c *harvesterProvider) ValidatePoolInfo(ctx context.Context, image string, flavor string, providerConfig string, extraspecs string) error {
	return nil
}

var _ execution.ExternalProvider = &harvesterProvider{}

func restConfigFromFile(kubeConfig string) (*rest.Config, error) {
	clientConfigPath, err := homedir.Expand(kubeConfig)
	if err != nil {
		return nil, err
	}

	clientConfig := kubeconfig.GetNonInteractiveClientConfig(clientConfigPath)
	return clientConfig.ClientConfig()
}

func restConfigFromBase64(kubeConfigBase64 string) (*rest.Config, error) {
	bytes, err := base64.StdEncoding.DecodeString(kubeConfigBase64)
	if err != nil {
		return nil, err
	}
	return clientcmd.RESTConfigFromKubeConfig(bytes)
}

func NewHarvesterProvider(providerConfig string, garmControllerId string) (execution.ExternalProvider, error) {
	var (
		restConfig *rest.Config
		err        error
	)

	var config Config
	if _, err := toml.DecodeFile(providerConfig, &config); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	kubeConfig := config.kubeConfig
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
	return &harvesterProvider{
		RestConfig:                restConfig,
		KubeVirtSubresourceClient: restClient,
		KubeClient:                kubeClient,
		StorageClassClient:        storageClassClient,
		HarvesterClient:           harvClient,
		HarvesterNetworkClient:    harvNetworkClient,
		ControllerID:              garmControllerId,
	}, nil
}
