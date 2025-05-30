package main

import (
	"garm-provider-harvester/pkg/utils"
	"log/slog"
	"testing"
)

func TestMapEnum(t *testing.T) {
	r, _ := utils.NewGarmCommand("CreateInstance")
	slog.Info("Win:", r)
}
