package kustomize

import (
	"bytes"
	"sort"

	"gopkg.in/yaml.v3"
)

type File struct {
	APIVersion string   `yaml:"apiVersion,omitempty"`
	Kind       string   `yaml:"kind,omitempty"`
	Resources  []string `yaml:"resources,omitempty"`
}

func Parse(content []byte) (File, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return defaultFile(), nil
	}

	var file File
	if err := yaml.Unmarshal(content, &file); err != nil {
		return File{}, err
	}

	if file.APIVersion == "" {
		file.APIVersion = "kustomize.config.k8s.io/v1beta1"
	}
	if file.Kind == "" {
		file.Kind = "Kustomization"
	}

	return file, nil
}

func EnsureResource(content []byte, resource string) ([]byte, error) {
	file, err := Parse(content)
	if err != nil {
		return nil, err
	}

	for _, existing := range file.Resources {
		if existing == resource {
			return Marshal(file)
		}
	}

	file.Resources = append(file.Resources, resource)
	sort.Strings(file.Resources)
	return Marshal(file)
}

func RemoveResource(content []byte, resource string) ([]byte, bool, error) {
	file, err := Parse(content)
	if err != nil {
		return nil, false, err
	}

	resources := file.Resources[:0]
	removed := false
	for _, existing := range file.Resources {
		if existing == resource {
			removed = true
			continue
		}
		resources = append(resources, existing)
	}
	file.Resources = resources

	out, err := Marshal(file)
	if err != nil {
		return nil, false, err
	}

	return out, removed, nil
}

func Marshal(file File) ([]byte, error) {
	out, err := yaml.Marshal(file)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func defaultFile() File {
	return File{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Resources:  []string{},
	}
}
