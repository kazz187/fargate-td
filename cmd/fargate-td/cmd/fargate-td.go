package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type FargateTdRunner struct {
	Debug bool
}

func NewFargateTdCommand() *cobra.Command {
	ftr := FargateTdRunner{}
	root := &cobra.Command{
		Use:              "fargate-td",
		Short:            "Manage task definition",
		Long:             "Manage task definition",
		Version:          "VERSION",
		PersistentPreRun: ftr.preRun,
	}
	root.AddCommand(VariablesCommand(&ftr))
	root.AddCommand(GenerateCommand(&ftr))
	root.AddCommand(DeployCommand(&ftr))
	root.AddCommand(WatchCommand(&ftr))
	return root
}

func (ftr *FargateTdRunner) preRun(c *cobra.Command, args []string) {
	logrus.SetOutput(os.Stderr)
	if ftr.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
