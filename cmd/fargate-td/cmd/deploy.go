package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/google/go-cmp/cmp"
	"github.com/kazz187/fargate-td/internal/config"
	"github.com/logrusorgru/aurora"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func DeployCommand(ftr *FargateTdRunner) *cobra.Command {
	r := &DeployRunner{
		GenerateRunner: GenerateRunner{
			VariablesRunner: *NewVariablesRunner(),
		},
	}
	c := &cobra.Command{
		Use:   `deploy -p PATH -t TASK -v"Key=Value" -n TASK_DEFINITION_NAME`,
		Short: "Deploy task definition",
		Long: `Deploy task definition

Run 'fargate-td deploy -p PATH -t TASK -v"Key=Value -n TASK_DEFINITION_NAME"

    $ fargate-td deploy -p app1/development -t task1 -v"Version=0.0.1"`,
		PreRunE: r.preRunE,
		RunE:    r.runE,
	}
	SetGenerateOptions(c, ftr, &r.GenerateRunner)
	c.Flags().Bool("td-only", false, "deploy task definition only")
	r.Command = c
	return c
}

type DeployRunner struct {
	GenerateRunner
	TdOnly bool
}

func (r *DeployRunner) preRunE(c *cobra.Command, args []string) error {
	err := r.GenerateRunner.preRunE(c, args)
	if err != nil {
		return err
	}
	return nil
}

func (r *DeployRunner) runE(c *cobra.Command, args []string) error {
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
	taskConfList := deployConf.GetTaskConfig(r.TaskName)
	taskStr, err := r.GenerateRunner.GenerateTaskDefinition()
	if err != nil {
		return err
	}
	taskYaml := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(taskStr), &taskYaml)
	if err != nil {
		return fmt.Errorf("failed to unmarshal task yaml")
	}
	replacedTaskYaml := replaceLowerCaseKey(taskYaml)
	inStr, err := yaml.Marshal(replacedTaskYaml)
	if err != nil {
		return fmt.Errorf("failed to marshal task yaml")
	}
	in := &ecs.RegisterTaskDefinitionInput{}
	err = yaml.Unmarshal(inStr, in)
	if err != nil {
		return fmt.Errorf("failed to load task definition yaml file: %w", err)
	}
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return fmt.Errorf("failed to load aws config: %w", err)
	}
	svc := ecs.New(cfg)
	tdRes, err := svc.RegisterTaskDefinitionRequest(in).Send(context.Background())
	if err != nil {
		return fmt.Errorf("failed to register task definition: %w", err)
	}
	err = diffTaskDefinition(svc, taskConfList, tdRes.TaskDefinition)
	if err != nil {
		return fmt.Errorf("failed to ")
	}
	if r.TdOnly {
		err := updateService(svc, taskConfList, *tdRes.TaskDefinition.TaskDefinitionArn)
		if err != nil {
			return err
		}
	}
	return nil
}

func diffTaskDefinition(svc *ecs.Client, taskConfList []config.TaskConfig, newTd *ecs.TaskDefinition) error {

	clusters := map[string][]string{}
	for _, taskConf := range taskConfList {
		c, ok := clusters[taskConf.Cluster]
		if ok {
			clusters[taskConf.Cluster] = append(c, taskConf.Service)
		} else {
			clusters[taskConf.Cluster] = []string{taskConf.Service}
		}
	}

	for cluster, services := range clusters {
		svcRes, err := svc.DescribeServicesRequest(&ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: services,
		}).Send(context.Background())
		if err != nil {
			return err
		}
		if svcRes == nil {
			return fmt.Errorf("service is not found in cluster %s", cluster)
		}
		svcToTd := map[string]string{}
		for _, s := range svcRes.Services {
			svcToTd[*s.ServiceName] = *s.TaskDefinition
		}

		for _, s := range services {
			fmt.Printf("Deploy to [cluster: %s, service: %s]\n", cluster, s)
			td, ok := svcToTd[s]
			if !ok {
				return fmt.Errorf("service %s is not found in cluster %s", s, cluster)
			}
			currentTdRes, err := svc.DescribeTaskDefinitionRequest(&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &td,
			}).Send(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get current task definition: %w", err)
			}
			currentTdRes.TaskDefinition.Revision = newTd.Revision
			currentTdRes.TaskDefinition.TaskDefinitionArn = newTd.TaskDefinitionArn
			diff := cmp.Diff(currentTdRes.TaskDefinition, newTd)
			if diff == "" {
				fmt.Println("already up-to-date")
			} else {
				fmt.Println("```")
				displayColorDiff(diff)
				fmt.Println("```")
			}
		}
	}
	return nil
}

func updateService(svc *ecs.Client, taskConfList []config.TaskConfig, tdArn string) error {
	var failedServiceList []string
	for _, taskConf := range taskConfList {
		serviceInput := &ecs.UpdateServiceInput{
			Cluster:        &taskConf.Cluster,
			Service:        &taskConf.Service,
			TaskDefinition: &tdArn,
		}
		_, err := svc.UpdateServiceRequest(serviceInput).Send(context.Background())
		if err != nil {
			logrus.Errorf("failed to update service: %w", err)
			failedServiceList = append(failedServiceList, "[cluster: "+taskConf.Cluster+", service: "+taskConf.Service+"]")
		}
	}
	if len(failedServiceList) != 0 {
		return fmt.Errorf("failed to update services: %s", strings.Join(failedServiceList, ", "))
	}
	return nil
}

func replaceLowerCaseKey(data map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range data {
		lowerK := strings.ToLower(k)
		switch v.(type) {
		case map[string]interface{}:
			result[lowerK] = replaceLowerCaseKey(v.(map[string]interface{}))
		case []interface{}:
			result[lowerK] = visitElements(v.([]interface{}))
		default:
			result[lowerK] = v
		}
	}
	return result
}

func visitElements(data []interface{}) []interface{} {
	var result []interface{}
	for _, e := range data {
		switch e.(type) {
		case map[string]interface{}:
			result = append(result, replaceLowerCaseKey(e.(map[string]interface{})))
		case []interface{}:
			result = append(result, visitElements(e.([]interface{})))
		default:
			result = append(result, e)
		}
	}
	return result
}

func displayColorDiff(diff string) {
	for _, s := range strings.Split(diff, "\n") {
		if strings.HasPrefix(s, "+") {
			fmt.Println(aurora.Green(s))
		} else if strings.HasPrefix(s, "-") {
			fmt.Println(aurora.Red(s))
		} else {
			fmt.Println(s)
		}
	}
}
