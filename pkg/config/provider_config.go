package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Credentials struct {
	KubeConfig string `toml:"kubeconfig"`
}

func (c Credentials) Validate() error {
	if c.KubeConfig == "" {
		return fmt.Errorf("missing kubeconfig")
	}

	_, err := base64.StdEncoding.DecodeString(c.KubeConfig); if err == nil {
		return nil
	}
	if _, err := os.Stat(c.KubeConfig); err != nil {
		return fmt.Errorf("kubeconfig %.80s does not exist or is not valid base64", c.KubeConfig)
	}

	return nil
}

// TODO: Add disk size to override VM image size disk.
type Config struct {
	Credentials      Credentials `toml:"credentials"`
	Namespace        string      `toml:"namespace"`
}

func NewProviderConfig(providerConfig string) (config Config, err error) {
	if _, err := toml.DecodeFile(providerConfig, &config); err != nil {
		return Config{}, fmt.Errorf("error decoding config: %w", err)
	}

	return config, nil
}

func (c *Config) Validate() error {
	if err := c.Credentials.Validate(); err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}

	if c.Namespace == "" {
		return fmt.Errorf("missing namespaces")
	}

	return nil
}

// GetConfigJSONSchema implements executionv011.ExternalProvider.
func (c *Config) GetConfigJSONSchema(ctx context.Context) (string, error) {
	return "", nil
}

// GetExtraSpecsJSONSchema implements executionv011.ExternalProvider.
func (c *Config) GetExtraSpecsJSONSchema(ctx context.Context) (string, error) {
	return "", nil
}

// GetSupportedInterfaceVersions implements executionv011.ExternalProvider.
func (c *Config) GetSupportedInterfaceVersions(ctx context.Context) []string {
	return []string{"v0.1.0", "v0.1.1"}
}

// ValidatePoolInfo implements executionv011.ExternalProvider.
func (c *Config) ValidatePoolInfo(ctx context.Context, image string, flavor string, providerConfig string, extraspecs string) error {
	return nil
}
