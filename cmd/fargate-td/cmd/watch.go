package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kazz187/fargate-td/pkg/watch"

	"github.com/kazz187/fargate-td/internal/config"

	"github.com/spf13/cobra"
)

func WatchCommand(ftr *FargateTdRunner) *cobra.Command {
	r := &WatchRunner{}
	c := &cobra.Command{
		Use:   `watch -p PATH -t TASK -v"Key=Value"`,
		Short: "Watch deployment status",
		Long: `Watch deployment status

Run 'fargate-td watch -p PATH -t TASK

    $ fargate-td watch -p app1/development -t task1`,
		PreRunE: r.preRunE,
		RunE:    r.runE,
	}
	SetWatchOptions(c, ftr, r)
	r.Command = c
	return c
}

func SetWatchOptions(c *cobra.Command, ftr *FargateTdRunner, r *WatchRunner) {
	c.Flags().StringVarP(&r.TaskName, "task", "t", "", "task name")
	_ = c.MarkFlagRequired("task")
	c.Flags().StringVarP(&r.TargetTaskPath, "path", "p", "", "watch target path")
	_ = c.MarkFlagRequired("path")
	c.Flags().StringVarP(&r.ProjectRootPath, "root_path", "r", "", "project root path")
	c.Flags().BoolVarP(&ftr.Debug, "debug", "d", false, "debug option")
}

type WatchRunner struct {
	Command         *cobra.Command
	TaskName        string
	TargetTaskPath  string
	ProjectRootPath string
}

func (r *WatchRunner) preRunE(c *cobra.Command, args []string) error {
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

func (r *WatchRunner) runE(c *cobra.Command, args []string) error {
	deployConf := config.NewDeployConfig()
	searchDeployConfPath := filepath.Clean(
		strings.Join(
			[]string{
				r.ProjectRootPath,
				taskPath,
				r.TargetTaskPath,
			},
			"/",
		),
	)
	err := deployConf.Load(searchDeployConfPath)
	if err != nil {
		return err
	}
	servicesMap := deployConf.GetServicesMapGroupByCluster(r.TaskName)
	for cluster, services := range servicesMap {
		r.Watch(cluster, services)
	}

	return nil
}

func (r *WatchRunner) Watch(cluster string, services []string) {
	interval := 10 * time.Second
	timeout := 10 * time.Minute
	w := watch.NewWatch(cluster, services, interval, timeout)
	go w.Start()
	for result := range w.Results {
		switch result.Status {
		case watch.DEPLOYED:
			fmt.Printf("Deployed [cluster: %s, service: %s]\n", result.Cluster, result.Service)
		case watch.DEPLOY_FAILED:
			fmt.Printf("Failed to deploy [cluster: %s, service: %s]: %s\n", result.Cluster, result.Service, result.Error.Error())
		case watch.ERROR:
			fmt.Printf("Error [cluster: %s, service: %s]: %s\n", result.Cluster, result.Service, result.Error.Error())
		case watch.TIMEOUT:
			fmt.Printf("Timeout [cluster: %s, service: %s]\n", result.Cluster, result.Service)
		}
	}
}
