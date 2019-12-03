package cmds

import (
	"github.com/kazz187/fargate-td/cmd/fargate-td/cmd"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:     "fargate-td",
	Short:   "Generate task definition",
	Long:    "Generate task definition",
	Version: "v0.0.1",
}

func NewFargateTdCommand(name string) *cobra.Command {
	root.AddCommand(cmd.GenerateCommand(name))

	return root
}
