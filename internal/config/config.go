package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	DefaultHarvesterNamespace = "default"
)

// Config holds the configuration for the Harvester provider.
type Config struct {
	// KubeconfigPath is the path to the kubeconfig file for accessing the Harvester cluster.
	// If empty, in-cluster authentication will be attempted.
	KubeconfigPath string `toml:"kubeconfig_path"`

	// KubeContext is the specific kubeconfig context to use.
	// If empty, the current context in the kubeconfig will be used.
	KubeContext string `toml:"kube_context"`

	// HarvesterNamespace is the default Kubernetes namespace where Harvester VMs and related
	// resources (like images, networks) are managed.
	// Defaults to "default" if not specified.
	HarvesterNamespace string `toml:"harvester_namespace"`

	// TODO: Add any other provider-specific settings here.
	// Example:
	// MaxInstancesPerPool int `toml:"max_instances_per_pool"`
	// DefaultVMFlavor string `toml:"default_vm_flavor"`
}

// LoadConfig reads the configuration file from the given filePath,
// parses it, sets default values, and returns the Config struct.
func LoadConfig(filePath string) (*Config, error) {
	var cfg Config

	if filePath == "" {
		// No config file path provided, use environment variables or defaults directly.
		// This can be an alternative to a config file for simpler setups.
		fmt.Println("No configuration file path provided. Using environment variables and defaults.")
		cfg.KubeconfigPath = os.Getenv("HARVESTER_KUBECONFIG_PATH") // Already used this pattern
		cfg.KubeContext = os.Getenv("HARVESTER_KUBECONFIG_CONTEXT")   // Already used this pattern
		cfg.HarvesterNamespace = os.Getenv("HARVESTER_NAMESPACE")
	} else {
		fmt.Printf("Loading configuration from file: %s\n", filePath)
		_, err := toml.DecodeFile(filePath, &cfg)
		if err != nil {
			// It might be acceptable for the config file to not exist if env vars are primary.
			// However, if a path is given, we should try to load it.
			// For now, let's be strict: if a path is given, it must be loadable.
			return nil, fmt.Errorf("failed to decode config file %s: %w", filePath, err)
		}
	}

	// Apply defaults if values are not set
	if cfg.HarvesterNamespace == "" {
		cfg.HarvesterNamespace = DefaultHarvesterNamespace
		fmt.Printf("HarvesterNamespace not set in config or env, using default: %s\n", DefaultHarvesterNamespace)
	}

	// Log loaded/defaulted config for verification (optional)
	fmt.Printf("Loaded configuration: KubeconfigPath='%s', KubeContext='%s', HarvesterNamespace='%s'\n",
		cfg.KubeconfigPath, cfg.KubeContext, cfg.HarvesterNamespace)

	return &cfg, nil
}
