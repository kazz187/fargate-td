package main

import (
	"os"

	"github.com/kazz187/fargate-td/cmd/fargate-td/cmd"
)

func main() {
	if err := cmd.NewFargateTdCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
