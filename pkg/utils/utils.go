package utils

import (
	"fmt"
	"os"

	"github.com/google/uuid"
)

type GarmCommand string

const (
	CreateInstance     GarmCommand = "CreateInstance"
	DeleteInstance     GarmCommand = "DeleteInstance"
	GetInstance        GarmCommand = "GetInstance"
	ListInstances      GarmCommand = "ListInstances"
	RemoveAllInstances GarmCommand = "RemoveAllInstances"
	Stop               GarmCommand = "Stop"
	Start              GarmCommand = "Start"
)

func NewGarmCommand(s string) (GarmCommand, error) {
	switch s {
	case string(CreateInstance):
		return CreateInstance, nil
	case string(DeleteInstance):
		return DeleteInstance, nil
	case string(GetInstance):
		return GetInstance, nil
	case string(ListInstances):
		return ListInstances, nil
	case string(RemoveAllInstances):
		return RemoveAllInstances, nil
	case string(Stop):
		return Stop, nil
	case string(Start):
		return Start, nil
	default:
		return "", fmt.Errorf("unknown GarmCommand string: %s", s)
	}
}

func ParseArgs() (garmCommand GarmCommand, garmProviderConfigFile string, garmControllerId uuid.UUID, garmPoolId *uuid.UUID, garmInstanceId *uuid.UUID) {
	garmCommandStr, exists := os.LookupEnv("GARM_COMMAND")
	if !exists {
		fmt.Println("[ERROR] GARM_COMMAND variable is required")
		os.Exit(1)
	}
	garmCommand, err := NewGarmCommand(garmCommandStr)
	if err != nil {
		fmt.Printf("[ERROR] Failed to parse %s must be: CreateInstance, DeleteInstance, GetInstance, ListInstances, RemoveAllInstances, Stop, or Start. %s\n", garmCommandStr, err)
		os.Exit(1)
	}
	fmt.Println("GARM_COMMAND:", garmCommand)

	garmProviderConfigFile, exists = os.LookupEnv("GARM_PROVIDER_CONFIG_FILE")
	if !exists {
		fmt.Println("[ERROR] GARM_PROVIDER_CONFIG_FILE variable is required")
		os.Exit(1)
	}
	_, err = os.Stat(garmProviderConfigFile)
	if err != nil {
		fmt.Printf("[ERROR] File path %s not found: %s\n", garmProviderConfigFile, exists)
	}
	fmt.Println("GARM_PROVIDER_CONFIG_FILE:", garmProviderConfigFile)

	garmControllerIdStr, exists := os.LookupEnv("GARM_CONTROLLER_ID")
	if !exists {
		fmt.Println("[ERROR] GARM_CONTROLLER_ID variable is required")
		os.Exit(1)
	}
	garmControllerId = uuid.MustParse(garmControllerIdStr)
	fmt.Println("GARM_CONTROLLER_ID:", garmControllerId)

	garmPoolId = nil
	garmPoolIdStr, exists := os.LookupEnv("GARM_POOL_ID")
	if exists {
		u := uuid.MustParse(garmPoolIdStr)
		garmPoolId = &u
		fmt.Println("GARM_POOL_ID:", garmPoolId)
	}

	garmInstanceId = nil
	garmInstanceIdStr, exists := os.LookupEnv("GARM_INSTANCE_ID")
	if exists {
		u := uuid.MustParse(garmInstanceIdStr)
		garmPoolId = &u
		fmt.Println("GARM_INSTANCE_ID:", garmPoolId)
	}

	return garmCommand, garmProviderConfigFile, garmControllerId, garmPoolId, garmInstanceId
}
