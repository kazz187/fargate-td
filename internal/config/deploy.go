package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/kazz187/fargate-td/internal/util"
)

type DeployConfig struct {
	serviceTaskConfig map[string][]ServiceTaskConfig
	cronJobTaskConfig map[string][]CronJobTaskConfig
}

type ServiceTaskConfig struct {
	Cluster string
	Service string
}

type CronJobTaskConfig struct {
	Cluster string
	CronJob string
	Cron    string
}

type config struct {
	Clusters []cluster `yaml:"clusters"`
}

type cluster struct {
	Name     string    `yaml:"name"`
	Services []service `yaml:"services"`
	CronJobs []cronJob `yaml:"cronJobs"`
}

type service struct {
	Name string `yaml:"name"`
	Task string `yaml:"task"`
}

type cronJob struct {
	Name string `yaml:"name"`
	Task string `yaml:"task"`
	Cron string `yaml:"cron"`
}

func NewDeployConfig() *DeployConfig {
	return &DeployConfig{
		serviceTaskConfig: map[string][]ServiceTaskConfig{},
		cronJobTaskConfig: map[string][]CronJobTaskConfig{},
	}
}

func (dc *DeployConfig) Load(searchPath string) error {
	configFile, err := searchConfigFile(searchPath)
	f, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read deploy config file %s: %w", configFile, err)
	}
	conf := &config{}
	err = yaml.Unmarshal(f, conf)
	if err != nil {
		return fmt.Errorf("failed to parse deploy config file %s: %w", configFile, err)
	}
	for _, c := range conf.Clusters {
		for _, s := range c.Services {
			taskConfigList, ok := dc.serviceTaskConfig[s.Task]
			if !ok {
				taskConfigList = []ServiceTaskConfig{}
			}
			taskConfigList = append(taskConfigList, ServiceTaskConfig{
				Cluster: c.Name,
				Service: s.Name,
			})
			dc.serviceTaskConfig[s.Task] = taskConfigList
		}
		for _, cj := range c.CronJobs {
			taskConfigList, ok := dc.cronJobTaskConfig[cj.Task]
			if !ok {
				taskConfigList = []CronJobTaskConfig{}
			}
			taskConfigList = append(taskConfigList, CronJobTaskConfig{
				Cluster: c.Name,
				CronJob: cj.Name,
				Cron:    cj.Cron,
			})
			dc.cronJobTaskConfig[cj.Task] = taskConfigList
		}
	}
	return nil
}

func searchConfigFile(searchPath string) (string, error) {
	tryFiles := []string{
		"config.yml",
		"config.yaml",
	}
	for _, f := range tryFiles {
		path := filepath.Clean(searchPath + "/" + f)
		if util.Exists(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("deploy config file is not found in %s", filepath.Clean(searchPath))
}

func (dc *DeployConfig) GetServiceTaskConfigs(task string) []ServiceTaskConfig {
	stc, ok := dc.serviceTaskConfig[task]
	if !ok {
		return []ServiceTaskConfig{}
	}
	return stc
}

func (dc *DeployConfig) GetCronJobTaskConfigs(task string) []CronJobTaskConfig {
	cjtc, ok := dc.cronJobTaskConfig[task]
	if !ok {
		return []CronJobTaskConfig{}
	}
	return cjtc
}

func (dc *DeployConfig) GetServicesMapGroupByCluster(task string) map[string][]string {
	serviceTaskConfList := dc.serviceTaskConfig[task]
	clusters := map[string][]string{}
	for _, taskConf := range serviceTaskConfList {
		c, ok := clusters[taskConf.Cluster]
		if ok {
			clusters[taskConf.Cluster] = append(c, taskConf.Service)
		} else {
			clusters[taskConf.Cluster] = []string{taskConf.Service}
		}
	}
	return clusters
}
