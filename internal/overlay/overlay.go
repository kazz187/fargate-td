package overlay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"

	"github.com/kazz187/fargate-td/internal/util"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/merge2"
)

const tplSuffix = ".tpl"

type Loader struct {
	RootPath   string
	TargetPath string
}

func NewLoader(rootPath string, targetPath string) *Loader {
	return &Loader{
		RootPath:   rootPath,
		TargetPath: targetPath,
	}
}

func (l *Loader) LoadOverlayTarget(targetName string, tplVars *yaml.RNode) (*yaml.RNode, error) {
	isTplMode := tplVars != nil
	targetFiles := l.searchTargetFiles(targetName, isTplMode)
	dst, err := mergeTargetFiles(targetFiles, tplVars)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

func mergeTargetFiles(targetFiles []string, tplVars *yaml.RNode) (*yaml.RNode, error) {
	tplVarsMap := map[string]interface{}{}
	loaded := false
	var dst *yaml.RNode
	for _, f := range targetFiles {
		var b []byte
		if strings.HasSuffix(f, tplSuffix) {
			tpl, err := template.ParseFiles(f)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template %s: %w", f, err)
			}
			if !loaded && tplVars != nil {
				err := tplVars.YNode().Decode(tplVarsMap)
				loaded = true
				if err != nil {
					return nil, fmt.Errorf("failed to convert tplVars: %w", err)
				}
			}
			buf := new(bytes.Buffer)
			err = tpl.Execute(buf, tplVarsMap)
			if err != nil {
				return nil, fmt.Errorf("failed to execute template %s: %w", f, err)
			}
			b = buf.Bytes()
		} else {
			var err error
			b, err = ioutil.ReadFile(f)
			if err != nil {
				return nil, fmt.Errorf("can't read a yaml file %s: %w", f, err)
			}
		}
		var err error
		dst, err = mergeStringYaml(string(b), dst)
		if err != nil {
			return nil, fmt.Errorf("failed to merge a yaml file %s: %w", f, err)
		}
	}
	return dst, nil
}

func mergeStringYaml(srcStr string, dst *yaml.RNode) (*yaml.RNode, error) {
	src, err := yaml.Parse(srcStr)
	if err != nil {
		if errors.Is(io.EOF, err) {
			src, _ = yaml.Parse("{}")
		} else {
			return nil, fmt.Errorf("can't parse string as yaml: %w", err)
		}
	}
	return merge2.Merge(src, dst)
}

func (l *Loader) searchTargetFiles(targetName string, isTplMode bool) []string {
	tryFiles := []string{
		targetName + ".yml",
		targetName + ".yaml",
	}
	if isTplMode {
		tryFiles = append([]string{
			targetName + ".yml" + tplSuffix,
			targetName + ".yaml" + tplSuffix,
		}, tryFiles...)
	}
	dirs := strings.Split(l.TargetPath, "/")
	path := l.RootPath
	var files []string
	for _, d := range dirs {
		path += d + "/"
		for _, tf := range tryFiles {
			tryPath := path + tf
			if util.Exists(tryPath) {
				files = append(files, tryPath)
				logrus.Debugln("file found:", tryPath)
				continue
			}
		}
	}
	return files
}
