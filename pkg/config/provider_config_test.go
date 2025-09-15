package config

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	f, err := os.CreateTemp("", "test-kube.yaml")
	require.NoError(t, err, "Failed to create temp file")
	defer os.Remove(f.Name())

	tests := []struct {
		name      string
		c         *Config
		errString string
	}{
		{
			name: "valid config kube file",
			c: &Config{
				Namespace: "test",
				Credentials: Credentials{
					KubeConfig: f.Name(),
				},
			},
			errString: "",
		},
		{
			name: "valid config kube file",
			c: &Config{
				Namespace: "test",
				Credentials: Credentials{
					KubeConfig: base64.StdEncoding.EncodeToString([]byte("hello")),
				},
			},
			errString: "",
		},
		{
			name: "missing NS",
			c: &Config{
				Credentials: Credentials{
					KubeConfig: base64.StdEncoding.EncodeToString([]byte("hello")),
				},
			},
			errString: "missing namespaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if tt.errString == "" {
				require.Nil(t, err)
			} else {
				require.EqualError(t, err, tt.errString)
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	f, err := os.CreateTemp("", "test-config.toml")
	require.NoError(t, err, "Failed to create temp file")
	defer os.Remove(f.Name())

	f.WriteString(`namespace = "garm-runners"

[credentials]
	kubeconfig = "/home/vscode/.kubeconfig"`)

	c, err := NewProviderConfig(f.Name())
	require.NoError(t, err, "Failed to create config struct")	

	require.Equal(t, c.Namespace, "garm-runners")
	require.Equal(t, c.Credentials.KubeConfig, "/home/vscode/.kubeconfig")

}