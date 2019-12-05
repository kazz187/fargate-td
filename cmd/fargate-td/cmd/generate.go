package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kazz187/fargate-td/internal/overlay"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const taskPath = "tasks"
const containerPath = "containers"

func GenerateCommand(ftr *FargateTdRunner) *cobra.Command {
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
	c.Flags().StringVarP(&r.TargetTaskPath, "path", "p", "", "generate target path")
	_ = c.MarkFlagRequired("path")
	c.Flags().StringVarP(&r.TaskName, "task", "t", "", "task name")
	_ = c.MarkFlagRequired("task")
	c.Flags().StringVarP(&r.ProjectRootPath, "root_path", "r", "", "project root path")
	c.Flags().StringToStringVarP(&r.Variables, "var", "v", map[string]string{}, "variables (key1=value1,key2=value2)")
	c.Flags().BoolVarP(&ftr.Debug, "debug", "d", false, "debug option")
	r.Command = c
	return c
}

type GenerateRunner struct {
	TargetTaskPath  string
	ProjectRootPath string
	TaskName        string
	Variables       map[string]string
	Command         *cobra.Command
}

func (r *GenerateRunner) preRunE(c *cobra.Command, args []string) error {
	if r.ProjectRootPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		r.ProjectRootPath = wd
	} else {
		var err error
		r.ProjectRootPath, err = filepath.Abs(r.ProjectRootPath)
		if err != nil {
			return err
		}
	}
	// Must contain prefix "/"
	r.TargetTaskPath = filepath.Clean("/" + r.TargetTaskPath)

	if strings.Contains(r.TaskName, "/") {
		return fmt.Errorf(`invalid task name (contains "/")`)
	}

	return nil
}

func (r *GenerateRunner) runE(c *cobra.Command, args []string) error {
	taskRootPath := r.ProjectRootPath + "/" + taskPath
	loader := overlay.NewLoader(taskRootPath, r.TargetTaskPath)
	configLoader := overlay.ConfigLoader{
		Loader:  loader,
		ArgVars: r.Variables,
	}
	conf, err := configLoader.LoadOverlayConfig()
	if err != nil {
		return fmt.Errorf("failed to load config files of task: %w", err)
	}
	task, err := loader.LoadOverlayTarget(r.TaskName, conf)
	if err != nil {
		return fmt.Errorf("failed to load task files %s: %w", r.TaskName, err)
	}
	confStr, err := conf.String()
	if err != nil {
		return fmt.Errorf("failed to convert yaml to string: %w", err)
	}
	taskStr, err := task.String()
	if err != nil {
		return fmt.Errorf("failed to convert yaml to string: %w", err)
	}
	logrus.Debugln(confStr)
	logrus.Debugln(taskStr)
	return nil
}
