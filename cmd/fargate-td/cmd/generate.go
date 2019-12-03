package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func GetGenerateRunner(name string) *GenerateRunner {
	r := &GenerateRunner{
		Variables: map[string]string{},
	}
	c := &cobra.Command{
		Use:   `generate -p PATH -t TASK -v"Key=Value"`,
		Short: "Generate task definition",
		Long: `Generate task definition

Run 'fargate-td generate -p PATH -t TASK -v"Key=Value"

    $ fargate-td generate -p app1/development -t task1 -v"Version=0.0.1"`,
		PreRunE: r.preRunE,
		RunE:    r.runE,
	}
	c.Flags().StringVarP(&r.Path, "path", "p", "", "generate path")
	_ = c.MarkFlagRequired("path")
	c.Flags().StringVarP(&r.Task, "task", "t", "", "task name")
	_ = c.MarkFlagRequired("task")
	c.Flags().StringVarP(&r.RootPath, "root_path", "r", "", "root path")
	c.Flags().StringToStringVarP(&r.Variables, "var", "v", map[string]string{}, "variables (key1=value1,key2=value2)")
	r.Command = c
	return r
}

func GenerateCommand(name string) *cobra.Command {
	return GetGenerateRunner(name).Command
}

type GenerateRunner struct {
	Path      string
	RootPath  string
	Task      string
	Variables map[string]string
	Command   *cobra.Command
}

func (r *GenerateRunner) preRunE(c *cobra.Command, args []string) error {
	if r.RootPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("can't get working directory: %w", err)
		}
		r.RootPath = wd
	}
	return nil
}

func (r *GenerateRunner) runE(c *cobra.Command, args []string) error {
	return nil
}
