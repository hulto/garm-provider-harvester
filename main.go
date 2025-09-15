package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"garm-provider-harvester/pkg/config"
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
	setupLogging()
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()

	executionEnv, err := execution.GetEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	slog.Info(fmt.Sprintf("executionEnv.ProviderConfigFile: %s", executionEnv.ProviderConfigFile))

	provConfig, err := config.NewProviderConfig(executionEnv.ProviderConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	if (provConfig == config.Config{}) {
		log.Fatalf("%s created an empty config", executionEnv.ProviderConfigFile)
	}

	prov, err := provider.NewHarvesterProvider(provConfig, executionEnv.ControllerID)
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
