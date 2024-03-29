package watch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/sirupsen/logrus"

	"github.com/kazz187/fargate-td/internal/util"
)

const (
	Deploying = iota
	Deployed
	DeployFailed
	Error
	Timeout
)

const statusRunning = "RUNNING"
const statusDeactivating = "DEACTIVATING"
const statusStopping = "STOPPING"
const statusDeprovisioning = "DEPROVISIONING"
const statusStopped = "STOPPED"

var failedStatuses = []string{
	statusDeactivating,
	statusStopping,
	statusDeprovisioning,
	statusStopped,
}

type Watch struct {
	Cluster           string
	Services          []string
	Interval, Timeout time.Duration
	Results           chan Result
}

type Result struct {
	Cluster string
	Service string
	Status  int
	Error   error
}

func NewWatch(cluster string, services []string, interval, timeout time.Duration) *Watch {
	return &Watch{
		Cluster:  cluster,
		Services: services,
		Interval: interval,
		Timeout:  timeout,
		Results:  make(chan Result, len(services)),
	}
}

func (w *Watch) Start() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		w.Results <- Result{
			Status: Error,
			Error:  fmt.Errorf("failed to load aws config: %w", err),
		}
		close(w.Results)
		return
	}

	ecsService := ecs.NewFromConfig(cfg)
	descServicesIn := &ecs.DescribeServicesInput{
		Cluster:  &w.Cluster,
		Include:  nil,
		Services: w.Services,
	}
	descServices, err := ecsService.DescribeServices(ctx, descServicesIn)
	if err != nil {
		w.Results <- Result{
			Status: Error,
			Error:  fmt.Errorf("failed to describe services: %w", err),
		}
		close(w.Results)
		return
	}

	wg := sync.WaitGroup{}
	for _, service := range descServices.Services {
		wg.Add(1)
		go w.ticker(ctx, &wg, service, ecsService)
	}
	wg.Wait()
	close(w.Results)
}

func (w *Watch) check(ctx context.Context, ecsService *ecs.Client, service types.Service) Result {
	listTasksIn := &ecs.ListTasksInput{
		Cluster:     service.ClusterArn,
		ServiceName: service.ServiceName,
	}
	listTasks, err := ecsService.ListTasks(ctx, listTasksIn)
	if err != nil {
		return Result{
			Cluster: w.Cluster,
			Service: *service.ServiceName,
			Status:  Error,
			Error:   err,
		}
	}

	descTasksIn := &ecs.DescribeTasksInput{
		Cluster: service.ClusterArn,
		Tasks:   listTasks.TaskArns,
	}
	tasks, err := ecsService.DescribeTasks(ctx, descTasksIn)
	if err != nil {
		return Result{
			Cluster: w.Cluster,
			Service: *service.ServiceName,
			Status:  Error,
			Error:   err,
		}
	}
	runningCount := int64(0)
	for _, task := range tasks.Tasks {
		logrus.Debugln("Service: " + *service.TaskDefinition)
		logrus.Debugln("Task: " + *task.TaskDefinitionArn)
		logrus.Debugln("DesiredStatus: " + *task.DesiredStatus)
		logrus.Debugln("LastStatus: " + *task.LastStatus)
		if *task.TaskDefinitionArn != *service.TaskDefinition {
			continue
		}
		if util.ContainsString(failedStatuses, *task.LastStatus) {
			return Result{
				Cluster: w.Cluster,
				Service: *service.ServiceName,
				Status:  DeployFailed,
				Error:   errors.New("task status is " + *task.LastStatus),
			}
		}
		if *task.DesiredStatus != statusRunning || *task.LastStatus != statusRunning {
			return Result{
				Cluster: w.Cluster,
				Service: *service.ServiceName,
				Status:  Deploying,
				Error:   nil,
			}
		}
		runningCount++
	}
	if int32(runningCount) != service.DesiredCount {
		return Result{
			Cluster: w.Cluster,
			Service: *service.ServiceName,
			Status:  Deploying,
			Error:   nil,
		}
	}
	return Result{
		Cluster: w.Cluster,
		Service: *service.ServiceName,
		Status:  Deployed,
		Error:   nil,
	}

}

func (w *Watch) ticker(ctx context.Context, wg *sync.WaitGroup, service types.Service, ecsService *ecs.Client) {
	defer wg.Done()
	ticker := time.NewTicker(w.Interval)
	timer := time.NewTimer(w.Timeout)
	defer func() {
		ticker.Stop()
		timer.Stop()
	}()

CHECK:
	result := w.check(ctx, ecsService, service)

	switch result.Status {
	case Deployed, DeployFailed, Error:
		w.Results <- result
		return
	case Deploying:
		// noop
	}

	select {
	case <-ticker.C:
		goto CHECK
	case <-timer.C:
		logrus.Errorf("timeout [cluster: %s, service: %s]", w.Cluster, *service.ServiceName)
		w.Results <- Result{
			Cluster: w.Cluster,
			Service: *service.ServiceName,
			Status:  Timeout,
			Error:   nil,
		}
		return
	}
}
