package main

import (
	"fmt"
	"garm-provider-harvester/pkg/provider"
	"garm-provider-harvester/pkg/utils"
)

func main() {
	garmCommand, garmProviderConfigFile, garmControllerId, garmPoolId, garmInstanceId := utils.ParseArgs()
	harvesterProvider := provider.HarvestProvider{
		ControllerID: garmControllerId,
	}
	switch garmCommand {
	case "CreateInstance":
		// TODO: Implement CreateInstance
	case "DeleteInstance":
		// TODO: Implement DeleteInstance
	case "GetInstance":
		// TODO: Implement GetInstance
	case "ListInstances":
		harvesterProvider.ListInstances()
		// TODO: Implement ListInstances
	case "StartInstance":
		// TODO: Implement StartInstance
	case "StopInstance":
		// TODO: Implement StopInstance
	default:
		fmt.Println("Unknown command:", garmCommand)
	}
	fmt.Println("Inputs: ", garmCommand, garmProviderConfigFile, garmControllerId, garmPoolId, garmInstanceId)
}
