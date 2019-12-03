package config

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/kazz187/fargate-td/internal/util"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/merge2"
)

type Loader struct {
	Path      string
	RootPath  string
	Variables map[string]string
}

func NewLoader(path string, rootPath string, variables map[string]string) *Loader {
	return &Loader{
		Path:      path,
		RootPath:  rootPath,
		Variables: variables,
	}
}

func (l *Loader) Load() error {
	loadFiles := l.searchLoadFiles()
	var mergedConf *yaml.RNode
	for _, f := range loadFiles {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return fmt.Errorf("can't read a config file %s: %w", f, err)
		}
		mergedConf, err = mergeConfStr(string(b), mergedConf)
		if err != nil {
			return fmt.Errorf("can't merge a config file %s: %w", f, err)
		}
	}
	argConf := ""
	for k, v := range l.Variables {
		argConf += k + ": \"" + v + "\"\n"
	}
	mergedConf, err := mergeConfStr(argConf, mergedConf)
	if err != nil {
		return fmt.Errorf("can't merge command line arg config: %w", err)
	}

	str, err := mergedConf.String()
	if err != nil {
		return fmt.Errorf("can't convert config to yaml: %w", err)
	}
	println(str)
	return nil
}

func mergeConfStr(confStr string, mergedConf *yaml.RNode) (*yaml.RNode, error) {
	conf, err := yaml.Parse(confStr)
	if err != nil {
		if errors.Is(io.EOF, err) {
			conf, _ = yaml.Parse("{}")
		} else {
			return nil, fmt.Errorf("can't parse a config: %w", err)
		}
	}
	return merge2.Merge(conf, mergedConf)
}

func (l *Loader) searchLoadFiles() []string {
	const taskRootPath = "tasks"
	var tryFiles = []string{"config.yml", "config.yaml"}
	dirs := append([]string{taskRootPath}, strings.Split(l.Path, "/")...)
	path := l.RootPath
	var files []string
	for _, d := range dirs {
		path += "/" + d
		for _, tf := range tryFiles {
			tryPath := path + "/" + tf
			if util.Exists(tryPath) {
				files = append(files, tryPath)
				continue
			}
		}
	}
	return files
}
