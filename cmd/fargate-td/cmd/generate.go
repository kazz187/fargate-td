package cmd

import (
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/kazz187/fargate-td/internal/overlay"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const taskPath = "tasks"
const containerPath = "containers"

func GenerateCommand(ftr *FargateTdRunner) *cobra.Command {
	r := &GenerateRunner{
		VariablesRunner: *NewVariablesRunner(),
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
	c.Flags().StringVarP(&r.TaskName, "task", "t", "", "task name")
	_ = c.MarkFlagRequired("task")

	VariablesSetOptions(c, ftr, &r.VariablesRunner)
	r.Command = c
	return c
}

type GenerateRunner struct {
	VariablesRunner
	TaskName string
}

func (r *GenerateRunner) preRunE(c *cobra.Command, args []string) error {
	err := r.VariablesRunner.preRunE(c, args)
	if err != nil {
		return err
	}
	if strings.Contains(r.TaskName, "/") {
		return fmt.Errorf(`invalid task name (contains "/")`)
	}
	return nil
}

func (r *GenerateRunner) runE(c *cobra.Command, args []string) error {
	conf, err := r.VariablesRunner.LoadVariables()
	if err != nil {
		return err
	}

	taskRootPath := r.ProjectRootPath + "/" + taskPath
	loader := overlay.NewLoader(taskRootPath, r.TargetTaskPath)
	task, err := loader.LoadOverlayTarget(r.TaskName, conf)
	if err != nil {
		return fmt.Errorf("failed to load task files %s: %w", r.TaskName, err)
	}
	containerRootPath := r.ProjectRootPath + "/" + containerPath
	cl := overlay.NewContainerLoader(containerRootPath, conf)
	if task == nil || task.YNode().Kind != yaml.MappingNode {
		return errors.New("task is not map")
	}
	cd := task.Field("containerDefinitions")
	if cd.Value == nil || cd.Value.YNode().Kind != yaml.SequenceNode {
		return errors.New("containerDefinition is not list")
	}
	err = cd.Value.VisitElements(func(conDef *yaml.RNode) error {
		if conDef == nil || conDef.YNode().Kind != yaml.MappingNode {
			return nil
		}
		conName := conDef.Field("template").Value.YNode().Value
		c, err := cl.LoadContainer(conName)
		if err != nil {
			return fmt.Errorf("failed to load container definition: %w", err)
		}
		conDef.SetYNode(c.YNode())
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}
	confStr, err := conf.String()
	if err != nil {
		return fmt.Errorf("failed to convert yaml to string: %w", err)
	}
	taskStr, err := task.String()
	if err != nil {
		return fmt.Errorf("failed to convert yaml to string: %w", err)
	}
	logrus.Debugln("generated variables:", confStr)
	fmt.Print(taskStr)
	return nil
}
