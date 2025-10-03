package config

import (
	"fmt"

	"github.com/harvester/harvester/pkg/builder"
)

type HarvesterExtraSpec struct {
	NetworkName string `json:"network_name,omitempty"`
	NetworkAdapterType string `json:"network_adapter_type,omitempty"`
	NetworkType string `json:"network_type,omitempty"`
	DiskConnectorType string `json:"disk_connector_type,omitempty"`
}

func (h HarvesterExtraSpec) Validate() error {
	if h.NetworkType != "" {
		if h.NetworkType != "bridge" && 
				h.NetworkType != "masquerade" {
			return fmt.Errorf("invalid network_type: %s", h.NetworkType)
		}
	}
	if h.NetworkAdapterType != "" {
		if h.NetworkAdapterType != "virtio" && 
				h.NetworkAdapterType != "e1000" && 
				h.NetworkAdapterType != "e1000e" && 
				h.NetworkAdapterType != "pcnet" && 
				h.NetworkAdapterType != "ne2k_pci" && 
				h.NetworkAdapterType != "rtl8139" {
			return fmt.Errorf("invalid network_adapter_type: %s", h.NetworkAdapterType)
		}
	}
	if h.DiskConnectorType != "" {
		if h.DiskConnectorType != builder.DiskBusVirtio &&
				h.DiskConnectorType != builder.DiskBusSata &&
				h.DiskConnectorType != builder.DiskBusScsi {
			return fmt.Errorf("invalid disk_connector_type: %s", h.DiskConnectorType)
		}
	}

	return nil
}