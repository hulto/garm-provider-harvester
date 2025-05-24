package harvester

import (
	"fmt"

	harvestersdk "github.com/harvester/harvester-sdk-go/pkg/client"
	"k8s.io/client-go/tools/clientcmd"
)

// HarvesterClient wraps the Harvester API client.
type HarvesterClient struct {
	client *harvestersdk.HarvesterClient
}

// NewHarvesterClient creates a new Harvester client from the given kubeconfig path and context.
// If kubeconfigPath is empty, it will attempt to use in-cluster config.
func NewHarvesterClient(kubeconfigPath string, kubeContext string) (*HarvesterClient, error) {
	if kubeconfigPath == "" {
		// Attempt to use in-cluster config if no kubeconfigPath is provided
		// This is useful when the provider runs inside a Kubernetes pod within the Harvester cluster
		cfg, err := clientcmd.BuildConfigFromFlags("", "") // "" for master URL and kubeconfig path for in-cluster
		if err != nil {
			return nil, fmt.Errorf("failed to build in-cluster kubeconfig: %w", err)
		}

		harvesterClient, err := harvestersdk.NewHarvesterClientFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create harvester client from in-cluster config: %w", err)
		}
		return &HarvesterClient{client: harvesterClient}, nil
	}

	// Use explicit kubeconfig path and context
	clientConfigLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientConfigLoadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config from kubeconfig: %w", err)
	}

	harvesterClient, err := harvestersdk.NewHarvesterClientFromConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create harvester client: %w", err)
	}

	return &HarvesterClient{client: harvesterClient}, nil
}

// API returns the underlying Harvester SDK client.
func (hc *HarvesterClient) API() *harvestersdk.HarvesterClient {
	return hc.client
}
