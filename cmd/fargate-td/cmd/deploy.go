package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go/document"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/logrusorgru/aurora/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kazz187/fargate-td/internal/config"
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
	c.Flags().BoolVar(&r.TdOnly, "td-only", false, "deploy task definition only")
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
	ctx := context.Background()
	// Load deploy config
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

	// Generate task definition
	taskStr, err := r.GenerateRunner.GenerateTaskDefinition()
	if err != nil {
		return err
	}

	// Replace keys of task yaml to lowercase
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

	// Load to struct RegisterTaskDefinitionInput
	in := &ecs.RegisterTaskDefinitionInput{}
	err = yaml.Unmarshal(inStr, in)
	if err != nil {
		return fmt.Errorf("failed to load task definition yaml file: %w", err)
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load aws config: %w", err)
	}

	//
	ecsSvc := ecs.NewFromConfig(cfg)
	cweSvc := cloudwatchevents.NewFromConfig(cfg)
	tdRes, err := ecsSvc.RegisterTaskDefinition(ctx, in)
	if err != nil {
		return fmt.Errorf("failed to register task definition: %w", err)
	}
	servicesMap := deployConf.GetServicesMapGroupByCluster(r.TaskName)
	serviceDiffMap, err := diffServiceTaskDefinition(ctx, ecsSvc, servicesMap, tdRes.TaskDefinition)
	if err != nil {
		return fmt.Errorf("failed to compare task definitions: %w", err)
	}
	cronJobs := deployConf.GetCronJobTaskConfigs(r.TaskName)
	cronJobDiffMap, err := diffCronJobTaskDefinition(ctx, ecsSvc, cweSvc, cronJobs, tdRes.TaskDefinition)
	if !r.TdOnly {
		serviceTaskConfig := deployConf.GetServiceTaskConfigs(r.TaskName)
		if err := updateService(ctx, ecsSvc, serviceTaskConfig, serviceDiffMap, *tdRes.TaskDefinition.TaskDefinitionArn); err != nil {
			return err
		}

		cronJobTaskConfig := deployConf.GetCronJobTaskConfigs(r.TaskName)
		if err := updateCronJob(ctx, cweSvc, cronJobTaskConfig, cronJobDiffMap, *tdRes.TaskDefinition.TaskDefinitionArn); err != nil {
			return err
		}
	}
	return nil
}

func diffServiceTaskDefinition(ctx context.Context, svc *ecs.Client, servicesMap map[string][]string, newTd *types.TaskDefinition) (map[string]string, error) {
	diffMap := map[string]string{}

	for cluster, services := range servicesMap {
		svcRes, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: services,
		})
		if err != nil {
			return nil, err
		}
		if svcRes == nil {
			return nil, fmt.Errorf("service is not found in cluster %s", cluster)
		}
		svcToTd := map[string]string{}
		for _, s := range svcRes.Services {
			svcToTd[*s.ServiceName] = *s.TaskDefinition
		}

		for _, s := range services {
			fmt.Printf("Diff [cluster: %s, service: %s]\n", cluster, s)
			td, ok := svcToTd[s]
			if !ok {
				return nil, fmt.Errorf("service %s is not found in cluster %s", s, cluster)
			}
			currentTdRes, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &td,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get current task definition: %w", err)
			}
			currentTdRes.TaskDefinition.Revision = newTd.Revision
			currentTdRes.TaskDefinition.TaskDefinitionArn = newTd.TaskDefinitionArn
			diff := cmp.Diff(currentTdRes.TaskDefinition, newTd, cmpopts.IgnoreTypes(document.NoSerde{}), cmpopts.IgnoreFields(*newTd, "RegisteredAt"))
			diffMap[s] = diff
			if diff == "" {
				fmt.Println("Already up-to-date")
			} else {
				fmt.Println("```")
				displayColorDiff(diff)
				fmt.Println("```")
			}
		}
	}
	return diffMap, nil
}

func diffCronJobTaskDefinition(ctx context.Context, ecsSvc *ecs.Client, cweSvc *cloudwatchevents.Client, cronJobs []config.CronJobTaskConfig, newTd *types.TaskDefinition) (map[string]string, error) {
	diffMap := map[string]string{}

	for _, job := range cronJobs {
		fmt.Printf("Diff [cluster: %s, cronJob: %s]\n", job.Cluster, job.CronJob)
		rule, err := cweSvc.DescribeRule(ctx, &cloudwatchevents.DescribeRuleInput{
			Name: &job.CronJob,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get rule: %w", err)
		}
		targets, err := cweSvc.ListTargetsByRule(ctx, &cloudwatchevents.ListTargetsByRuleInput{
			Rule: rule.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get targets: %w", err)
		}
		for _, target := range targets.Targets {
			if target.EcsParameters == nil {
				continue
			}
			tdArn := target.EcsParameters.TaskDefinitionArn
			currentTd, err := ecsSvc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: tdArn,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get task definition: %w", err)
			}
			currentTd.TaskDefinition.Revision = newTd.Revision
			currentTd.TaskDefinition.TaskDefinitionArn = newTd.TaskDefinitionArn
			diff := cmp.Diff(currentTd.TaskDefinition, newTd, cmpopts.IgnoreTypes(document.NoSerde{}), cmpopts.IgnoreFields(*newTd, "RegisteredAt"))
			diffMap[*target.Id] = diff
			if diff == "" {
				fmt.Println("Already up-to-date")
			} else {
				fmt.Println("```")
				displayColorDiff(diff)
				fmt.Println("```")
			}
		}
	}
	return diffMap, nil
}

func updateService(ctx context.Context, svc *ecs.Client, taskConfList []config.ServiceTaskConfig, diffMap map[string]string, tdArn string) error {
	var failedServiceList []string
	for _, taskConf := range taskConfList {
		if diffMap[taskConf.Service] == "" {
			fmt.Printf("Skip update service [cluster: %s, service: %s]\n", taskConf.Cluster, taskConf.Service)
			continue
		}
		fmt.Printf("Update service [cluster: %s, service: %s]\n", taskConf.Cluster, taskConf.Service)
		serviceInput := &ecs.UpdateServiceInput{
			Cluster:        &taskConf.Cluster,
			Service:        &taskConf.Service,
			TaskDefinition: &tdArn,
		}
		if _, err := svc.UpdateService(ctx, serviceInput); err != nil {
			logrus.Errorf("failed to update service: %s", err)
			failedServiceList = append(failedServiceList, "[cluster: "+taskConf.Cluster+", service: "+taskConf.Service+"]")
		}
	}
	if len(failedServiceList) != 0 {
		return fmt.Errorf("failed to update services: %s", strings.Join(failedServiceList, ", "))
	}
	return nil
}

func updateCronJob(ctx context.Context, cweSvc *cloudwatchevents.Client, taskConfList []config.CronJobTaskConfig, diffMap map[string]string, tdArn string) error {
	var failedCronJobList []string
	for _, taskConf := range taskConfList {
		ruleInput := cloudwatchevents.DescribeRuleInput{
			Name:         &taskConf.CronJob,
			EventBusName: nil,
		}
		rule, err := cweSvc.DescribeRule(ctx, &ruleInput)
		if err != nil {
			logrus.Errorf("failed to get cron rule: %s", err)
			failedCronJobList = append(failedCronJobList, "[cluster: "+taskConf.Cluster+", cron job: "+taskConf.CronJob+"]")
			continue
		}
		if *rule.ScheduleExpression != taskConf.Cron {
			cronJobInput := &cloudwatchevents.PutRuleInput{
				Name:               &taskConf.CronJob,
				ScheduleExpression: &taskConf.Cron,
			}
			fmt.Printf("Update cron schedule [%s -> %s]\n", *rule.ScheduleExpression, taskConf.Cron)
			if _, err := cweSvc.PutRule(ctx, cronJobInput); err != nil {
				logrus.Errorf("failed to update cron job: %s", err)
				failedCronJobList = append(failedCronJobList, "[cluster: "+taskConf.Cluster+", cron job: "+taskConf.CronJob+", cron: "+taskConf.Cron+"]")
			}
		}

		if diffMap[taskConf.CronJob] == "" {
			fmt.Printf("Skip update cron job [cluster: %s, cronJob: %s]\n", taskConf.Cluster, taskConf.CronJob)
			continue
		}

		listTargetsIn := cloudwatchevents.ListTargetsByRuleInput{
			Rule: rule.Name,
		}
		targets, err := cweSvc.ListTargetsByRule(ctx, &listTargetsIn)
		if err != nil {
			logrus.Errorf("failed to get targets: %s", err)
			failedCronJobList = append(failedCronJobList, "[cluster: "+taskConf.Cluster+", cron job: "+taskConf.CronJob+"]")
			continue
		}
		for i, target := range targets.Targets {
			if target.EcsParameters == nil {
				continue
			}
			if *target.EcsParameters.TaskDefinitionArn == tdArn {
				continue
			}
			fmt.Printf("Update cron job [cluster: %s, cronJob: %s, cron: %s]\n", taskConf.Cluster, taskConf.CronJob, taskConf.Cron)
			targets.Targets[i].EcsParameters.TaskDefinitionArn = &tdArn
		}
		putTargetsIn := &cloudwatchevents.PutTargetsInput{
			Rule:    &taskConf.CronJob,
			Targets: targets.Targets,
		}
		if _, err := cweSvc.PutTargets(ctx, putTargetsIn); err != nil {
			logrus.Errorf("failed to update target: %s", err)
			failedCronJobList = append(failedCronJobList, "[cluster: "+taskConf.Cluster+", cron job: "+taskConf.CronJob+"]")
		}
	}
	if len(failedCronJobList) != 0 {
		return fmt.Errorf("failed to update cron jobs: %s", strings.Join(failedCronJobList, ", "))
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
