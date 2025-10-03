package config

type HarvesterExtraSpec struct {
	NetworkName string `json:"network_name,omitempty"`
	NetworkAdapterType string `json:"network_adapter_type,omitempty"`
	DiskConnectorType string `json:"disk_connector_type,omitempty"`
}