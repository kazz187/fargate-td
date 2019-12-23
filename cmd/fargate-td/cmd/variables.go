package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/kazz187/fargate-td/internal/overlay"
	"github.com/spf13/cobra"
)

func VariablesCommand(ftr *FargateTdRunner) *cobra.Command {
	r := NewVariablesRunner()
	c := &cobra.Command{
		Use:   `variables -p PATH -v"Key=Value"`,
		Short: "Overlay variables",
		Long: `Overlay variables

Run 'fargate-td variables -p PATH -v"Key=Value"

    $ fargate-td variables -p app1/development -v"Version=0.0.1"`,
		PreRunE: r.preRunE,
		RunE:    r.runE,
	}
	r.Command = c
	SetVariablesOptions(c, ftr, r)
	return c
}

func SetVariablesOptions(c *cobra.Command, ftr *FargateTdRunner, r *VariablesRunner) {
	c.Flags().StringVarP(&r.TargetTaskPath, "path", "p", "", "generate target path")
	_ = c.MarkFlagRequired("path")
	c.Flags().StringVarP(&r.ProjectRootPath, "root_path", "r", "", "project root path")
	c.Flags().StringToStringVarP(&r.Variables, "var", "v", map[string]string{}, "variables (key1=value1,key2=value2)")
	c.Flags().BoolVarP(&ftr.Debug, "debug", "d", false, "debug option")
}

type VariablesRunner struct {
	TargetTaskPath  string
	ProjectRootPath string
	Variables       map[string]string
	Command         *cobra.Command
}

func NewVariablesRunner() *VariablesRunner {
	return &VariablesRunner{
		Variables: map[string]string{},
	}
}

func (r *VariablesRunner) preRunE(c *cobra.Command, args []string) error {
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
	return nil
}

func (r *VariablesRunner) runE(c *cobra.Command, args []string) error {
	vars, _ := r.LoadVariables()
	varsStr, err := vars.String()
	if err != nil {
		return fmt.Errorf("failed to convert yaml to string: %w", err)
	}
	fmt.Print(varsStr)
	return nil
}

func (r *VariablesRunner) LoadVariables() (*yaml.RNode, error) {
	taskRootPath := r.ProjectRootPath + "/" + taskPath
	loader := overlay.NewLoader(taskRootPath, r.TargetTaskPath)
	variablesLoader := overlay.VariablesLoader{
		Loader:  loader,
		ArgVars: r.Variables,
	}
	vars, err := variablesLoader.LoadOverlayVariables()
	if err != nil {
		return nil, fmt.Errorf("failed to load variables files of task: %w", err)
	}
	return vars, nil
}
