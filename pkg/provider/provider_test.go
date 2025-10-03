package provider

import (
	"garm-provider-harvester/pkg/config"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

// func TestGetBackingImage(t *testing.T) {
// 	h, err := NewHarvesterProvider(config.Config{
// 		Namespace: "garm-runners",
// 		Credentials: config.Credentials{
// 			KubeConfig: "/home/vscode/.kube/config",
// 		},
// 	}, "bebd05b9-18c6-4210-8b27-b669e266cbf1")
// 	if err != nil {
// 		log.Fatalf("Failed to load provider: %s", err)
// 	}

// 	harv := h.(*HarvesterProvider)
// 	res, err := harv.getBackingImageName(t.Context(), "harvester-public/ubuntu-server-noble-24.04")
// 	if err != nil {
// 		log.Fatalf("Failed to get backing image: %s", err)
// 	}
// 	require.Equal(t, "vmi-76663440-cf84-40ff-af02-508d7e27aea5", res)
// }

func TestGetStorageClass(t *testing.T) {
	h, err := NewHarvesterProvider(config.Config{
		Namespace: "garm-runners",
		Credentials: config.Credentials{
			KubeConfig: "/home/vscode/.kube/config",
		},
	}, "bebd05b9-18c6-4210-8b27-b669e266cbf1")
	if err != nil {
		log.Fatalf("Failed to load provider: %s", err)
	}

	harv := h.(*HarvesterProvider)
	res, err := harv.getStorageClass(t.Context(), "harvester-public/ubuntu-server-noble-24.04")
	if err != nil {
		log.Fatalf("Failed to get backing image: %s", err)
	}
	require.Equal(t, "longhorn-ubuntu-server-noble-24.04", res)
}