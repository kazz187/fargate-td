package overlay

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const configTarget = "config"

type ConfigLoader struct {
	*Loader
	ArgVars map[string]string
}

func (cl *ConfigLoader) LoadOverlayConfig() (*yaml.RNode, error) {
	conf, err := cl.Loader.LoadOverlayTarget(configTarget, nil)
	if err != nil {
		return nil, fmt.Errorf("can't load a config file: %w", err)
	}
	argVars := argVarsToStringYaml(cl.ArgVars)
	conf, err = mergeStringYaml(argVars, conf)
	if err != nil {
		return nil, fmt.Errorf("can't merge config from command line args: %w", err)
	}
	return conf, nil
}

func argVarsToStringYaml(argVars map[string]string) string {
	strYaml := ""
	for k, v := range argVars {
		strYaml += k + ": \"" + v + "\"\n"
	}
	return strYaml
}
