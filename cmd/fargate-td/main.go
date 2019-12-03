package main

import (
	"github.com/kazz187/fargate-td/cmd/fargate-td/cmds"
	"os"
)

func main() {
	if err := cmds.NewFargateTdCommand("").Execute(); err != nil {
		os.Exit(1)
	}
}
