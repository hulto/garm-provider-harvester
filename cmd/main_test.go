package main

import (
	"fmt"
	"garm-provider-harvester/pkg/utils"
	"testing"
)

func TestMapEnum(t *testing.T) {
	r, _ := utils.NewGarmCommand("CreateInstance")
	fmt.Println("Win:", r)
}
