package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/kazz187/fargate-td/internal/util"
	"gopkg.in/yaml.v3"
)

type DeployConfig struct {
	taskConfigs map[string][]TaskConfig
}

type TaskConfig struct {
	Cluster string
	Service string
}

type config struct {
	Clusters []cluster `yaml:"clusters"`
}

type cluster struct {
	Name     string    `yaml:"name"`
	Services []service `yaml:"services"`
}

type service struct {
	Name string `yaml:"name"`
	Task string `yaml:"task"`
}

func NewDeployConfig() *DeployConfig {
	return &DeployConfig{
		taskConfigs: map[string][]TaskConfig{},
	}
}

func (dc *DeployConfig) Load(searchPath string) error {
	configFile, err := searchConfigFile(searchPath)
	f, err := ioutil.ReadFile(configFile)
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
			taskConfigList, ok := dc.taskConfigs[s.Task]
			if !ok {
				taskConfigList = []TaskConfig{}
			}
			taskConfigList = append(taskConfigList, TaskConfig{
				Cluster: c.Name,
				Service: s.Name,
			})
			dc.taskConfigs[s.Task] = taskConfigList
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

func (dc *DeployConfig) GetTaskConfig(task string) []TaskConfig {
	return dc.taskConfigs[task]
}
