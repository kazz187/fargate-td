package overlay

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const variablesTarget = "variables"

type VariablesLoader struct {
	*Loader
	ArgVars map[string]string
}

func (cl *VariablesLoader) LoadOverlayVariables() (*yaml.RNode, error) {
	vars, err := cl.Loader.LoadOverlayTarget(variablesTarget, nil)
	if err != nil {
		return nil, fmt.Errorf("can't load a variables file: %w", err)
	}
	argVars := argVarsToStringYaml(cl.ArgVars)
	vars, err = mergeStringYaml(argVars, vars)
	if err != nil {
		return nil, fmt.Errorf("can't merge variables from command line args: %w", err)
	}
	return vars, nil
}

func argVarsToStringYaml(argVars map[string]string) string {
	strYaml := ""
	for k, v := range argVars {
		strYaml += k + ": \"" + v + "\"\n"
	}
	return strYaml
}
