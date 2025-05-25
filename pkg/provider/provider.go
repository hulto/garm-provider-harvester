package provider

import (
	"fmt"

	"github.com/google/uuid"
)

type HarvestProvider struct {
	// cfg          *config.Config
	// cli          *client.OpenstackClient
	ControllerID uuid.UUID
}

func (h HarvestProvider) ListInstances() error {
	fmt.Println("List")
	return nil
}
