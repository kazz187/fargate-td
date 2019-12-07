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
		Short:            "Generate task definition",
		Long:             "Generate task definition",
		Version:          "v0.0.1",
		PersistentPreRun: ftr.preRun,
	}
	root.AddCommand(GenerateCommand(&ftr))
	root.AddCommand(VariablesCommand(&ftr))
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
