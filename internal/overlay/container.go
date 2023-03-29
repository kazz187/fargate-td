package overlay

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/merge2"
)

type ContainerLoader struct {
	RootPath string
	TaskVars *yaml.RNode
}

func NewContainerLoader(rootPath string, taskVars *yaml.RNode) *ContainerLoader {
	return &ContainerLoader{
		RootPath: rootPath,
		TaskVars: taskVars,
	}
}

func (cl *ContainerLoader) LoadContainer(name string, taskConVars *yaml.RNode) (*yaml.RNode, error) {
	const containerTarget = "container"
	const variablesTarget = "variables"
	l := NewLoader(cl.RootPath, name)
	containerVars, err := l.LoadOverlayTarget(variablesTarget, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load variables %s: %w", name, err)
	}
	tplVars, err := merge2.Merge(cl.TaskVars, containerVars, yaml.MergeOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to merge variables %s: %w", name, err)
	}
	tplVars, err = merge2.Merge(taskConVars, tplVars, yaml.MergeOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to merge variables %s: %w", name, err)
	}
	container, err := l.LoadOverlayTarget(containerTarget, tplVars)
	if err != nil {
		return nil, fmt.Errorf("failed to load container %s: %w", name, err)
	}
	return container, nil
}
