package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"garm-provider-harvester/pkg/provider"

	"github.com/cloudbase/garm-provider-common/execution"
	commonExecution "github.com/cloudbase/garm-provider-common/execution/common"
)

func setupLogging() {
	handlerOptions := slog.HandlerOptions{Level: slog.LevelInfo}
	logHandler := slog.NewTextHandler(os.Stderr, &handlerOptions)
	logger := slog.New(logHandler)
	slog.SetDefault(logger)
}

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()

	executionEnv, err := execution.GetEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	prov, err := provider.NewHarvesterProvider(executionEnv.ProviderConfigFile, executionEnv.ControllerID)
	if err != nil {
		log.Fatal(err)
	}

	result, err := executionEnv.Run(ctx, prov)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run command: %s", err)
		os.Exit(commonExecution.ResolveErrorToExitCode(err))
	}
	if len(result) > 0 {
		fmt.Fprint(os.Stdout, result)
	}
}

// func main() {
// 	setupLogging()

// 	ctx := context.Background()
// 	garmCommand, garmProviderConfigFile, garmControllerId, garmPoolId, garmInstanceId := utils.ParseArgs()
// 	harvesterClient, err := provider.NewClient(garmProviderConfigFile, garmControllerId)
// 	if err != nil {
// 		slog.Info(fmt.Sprintf("[ERROR] Failed to initalize harvester client: %s\n", err))
// 		os.Exit(1)
// 	}
// 	switch garmCommand {
// 	case "CreateInstance":
// 		// TODO: Implement CreateInstance
// 	case "DeleteInstance":
// 		// TODO: Implement DeleteInstance
// 	case "GetInstance":
// 		// TODO: Implement GetInstance
// 	case "ListInstances":
// 		var vms []params.ProviderInstance
// 		vms, _ = harvesterClient.ListInstances(ctx)
// 		var res []byte
// 		res, err = json.Marshal(vms)
// 		slog.Info(string(res))
// 		// TODO: Implement ListInstances
// 	case "StartInstance":
// 		// TODO: Implement StartInstance
// 	case "StopInstance":
// 		// TODO: Implement StopInstance
// 	default:
// 		slog.Error(fmt.Sprintf("[ERROR] Unknown command: %s\n", garmCommand))
// 		err = fmt.Errorf("unknown command: %s", garmCommand)
// 	}
// 	if err != nil {
// 		slog.Error(fmt.Sprintf("[ERROR] Failed to list instances: ", err))
// 		os.Exit(1)
// 	}
// 	slog.Info(fmt.Sprintf("Inputs: %s %s %s %s %s", garmCommand, garmProviderConfigFile, garmControllerId, garmPoolId, garmInstanceId))
// }
