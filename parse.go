// Copyright 2020 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package yb

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yourbase/narwhal"
	"gopkg.in/yaml.v2"
)

type buildManifest struct {
	Dependencies dependencySet  `yaml:"dependencies"`
	Sandbox      bool           `yaml:"sandbox"`
	BuildTargets []*buildTarget `yaml:"build_targets"`
	Build        *buildTarget   `yaml:"build"`
	Exec         *execPhase     `yaml:"exec"`
	Package      *packagePhase  `yaml:"package"`
	CI           *ciInfo        `yaml:"ci"`
}

func parse(b []byte) (*Package, error) {
	manifest := new(buildManifest)
	if err := yaml.UnmarshalStrict(b, manifest); err != nil {
		return nil, err
	}
	pkg := new(Package)
	var err error
	pkg.Targets, err = parseTargets(pkg, manifest)
	if err != nil {
		return nil, err
	}
	pkg.ExecEnvironments, err = parseExecPhase(pkg, manifest)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

type buildTarget struct {
	Name         string               `yaml:"name"`
	Container    *containerDefinition `yaml:"container"`
	Commands     []string             `yaml:"commands"`
	HostOnly     bool                 `yaml:"host_only"`
	Root         string               `yaml:"root"`
	Environment  []string             `yaml:"environment"`
	Tags         map[string]string    `yaml:"tags"`
	BuildAfter   []string             `yaml:"build_after"`
	Dependencies buildDependencies    `yaml:"dependencies"`
}

type buildDependencies struct {
	Build      []string                        `yaml:"build"`
	Containers map[string]*containerDefinition `yaml:"containers"`
}

func parseTargets(pkg *Package, manifest *buildManifest) (map[string]*Target, error) {
	globalBuildDeps := make(map[string]BuildpackSpec)
	if err := parseBuildpacks(globalBuildDeps, manifest.Dependencies.Build); err != nil {
		return nil, fmt.Errorf("top-level build dependencies: %w", err)
	}

	// First pass: parse data attributes (things that don't involve references).
	targets := manifest.BuildTargets
	if manifest.Build != nil {
		// Old, single-target mechanism. Should be interpreted the same as
		// specifying a default target.
		manifest.Build.Name = DefaultTarget
		targets = append(targets, manifest.Build)
	}
	targetMap := make(map[string]*Target)
	for _, tgt := range targets {
		if targetMap[tgt.Name] != nil {
			return nil, fmt.Errorf("multiple targets with name %q", tgt.Name)
		}
		parsed, err := parseTarget(globalBuildDeps, tgt)
		if err != nil {
			return nil, err
		}
		parsed.Package = pkg
		targetMap[parsed.Name] = parsed
	}

	// Second pass: resolve target references.
	// We don't check for cycles at this point: that comes in validation.
	for _, tgt := range targets {
		if len(tgt.BuildAfter) > 0 {
			targetMap[tgt.Name].Deps = make(map[*Target]struct{})
		}
		for _, dep := range tgt.BuildAfter {
			found := targetMap[dep]
			if found == nil {
				return nil, fmt.Errorf("target %s: build_after: unknown target %q", tgt.Name, dep)
			}
			targetMap[tgt.Name].Deps[found] = struct{}{}
		}
	}

	return targetMap, nil
}

// parseTarget parses a target's data attributes (i.e. anything that doesn't
// refer to other targets).
func parseTarget(globalDeps map[string]BuildpackSpec, tgt *buildTarget) (*Target, error) {
	if tgt.Name == "" {
		return nil, errors.New("found target without name")
	}
	parsed := &Target{
		Name:       tgt.Name,
		Container:  &tgt.Container.toResource().ContainerDefinition,
		Commands:   tgt.Commands,
		RunDir:     tgt.Root,
		Tags:       tgt.Tags,
		Env:        make(map[string]EnvTemplate),
		Buildpacks: make(map[string]BuildpackSpec),
		HostOnly:   tgt.HostOnly,
		Resources:  makeResourceMap(tgt.Dependencies.Containers),
	}
	for tool, spec := range globalDeps {
		parsed.Buildpacks[tool] = spec
	}
	if err := parseBuildpacks(parsed.Buildpacks, tgt.Dependencies.Build); err != nil {
		return nil, fmt.Errorf("target %s: dependencies: build: %w", tgt.Name, err)
	}
	if err := parseEnv(parsed.Env, tgt.Environment); err != nil {
		return nil, fmt.Errorf("target %s: environment: %w", tgt.Name, err)
	}
	return parsed, nil
}

func parseBuildpacks(dst map[string]BuildpackSpec, list []string) error {
	for _, s := range list {
		spec, err := ParseBuildpackSpec(s)
		if err != nil {
			return err
		}
		dst[spec.Name()] = spec
	}
	return nil
}

type ciInfo struct {
	CIBuilds []*ciBuild `yaml:"builds"`
}

type ciBuild struct {
	Name         string `yaml:"name"`
	BuildTarget  string `yaml:"build_target"`
	When         string `yaml:"when"`
	ReportStatus bool   `yaml:"report_status"`
}

type packagePhase struct {
	Artifacts []string `yaml:"artifacts"`
}

type dependencySet struct {
	Build   []string `yaml:"build"`
	Runtime []string `yaml:"runtime"`
}

type execPhase struct {
	Name         string               `yaml:"name"`
	Dependencies execDependencies     `yaml:"dependencies"`
	Container    *containerDefinition `yaml:"container"`
	Commands     []string             `yaml:"commands"`
	Environment  map[string][]string  `yaml:"environment"`
	LogFiles     []string             `yaml:"logfiles"`
	Sandbox      bool                 `yaml:"sandbox"`
	HostOnly     bool                 `yaml:"host_only"`
}

type execDependencies struct {
	Containers map[string]*containerDefinition `yaml:"containers"`
}

func parseExecPhase(pkg *Package, manifest *buildManifest) (map[string]*Target, error) {
	if manifest.Exec == nil {
		return nil, nil
	}
	buildpacks := make(map[string]BuildpackSpec)
	if err := parseBuildpacks(buildpacks, manifest.Dependencies.Runtime); err != nil {
		return nil, fmt.Errorf("top-level runtime dependencies: %w", err)
	}
	defaultTarget := &Target{
		Name:       DefaultExecEnvironment,
		Package:    pkg,
		Container:  &manifest.Exec.Container.toResource().ContainerDefinition,
		Commands:   manifest.Exec.Commands,
		Env:        make(map[string]EnvTemplate),
		Buildpacks: buildpacks,
		HostOnly:   manifest.Exec.HostOnly,
		Resources:  makeResourceMap(manifest.Exec.Dependencies.Containers),
	}
	if err := parseEnv(defaultTarget.Env, manifest.Exec.Environment[defaultTarget.Name]); err != nil {
		return nil, fmt.Errorf("exec environment: %s: %w", defaultTarget.Name, err)
	}

	// Clone target for different environments.
	targetMap := make(map[string]*Target)
	targetMap[defaultTarget.Name] = defaultTarget
	for name, env := range manifest.Exec.Environment {
		if name == defaultTarget.Name {
			continue
		}
		newTarget := new(Target)
		*newTarget = *defaultTarget
		newTarget.Name = name
		newTarget.Env = make(map[string]EnvTemplate)
		for k, v := range defaultTarget.Env {
			newTarget.Env[k] = v
		}
		if err := parseEnv(newTarget.Env, env); err != nil {
			return nil, fmt.Errorf("exec environment: %s: %w", newTarget.Name, err)
		}
		targetMap[name] = newTarget
	}
	return targetMap, nil
}

// DefaultContainerImage is the Docker image used when none is specified.
const DefaultContainerImage = "yourbase/yb_ubuntu:18.04"

type containerDefinition struct {
	Image         string        `yaml:"image"`
	Mounts        []string      `yaml:"mounts"`
	Ports         []string      `yaml:"ports"`
	Environment   []string      `yaml:"environment"`
	Command       string        `yaml:"command"`
	WorkDir       string        `yaml:"workdir"`
	PortWaitCheck portWaitCheck `yaml:"port_check"`
	Label         string        `yaml:"label"`
}

func (def *containerDefinition) toResource() *ResourceDefinition {
	image := DefaultContainerImage
	if def == nil {
		return &ResourceDefinition{
			ContainerDefinition: narwhal.ContainerDefinition{
				Image: image,
			},
		}
	}
	if def.Image != "" {
		image = def.Image
	}
	return &ResourceDefinition{
		ContainerDefinition: narwhal.ContainerDefinition{
			Image:       image,
			Mounts:      append([]string(nil), def.Mounts...),
			Ports:       append([]string(nil), def.Ports...),
			Environment: append([]string(nil), def.Environment...),
			Argv:        strings.Fields(def.Command),
			WorkDir:     def.WorkDir,

			HealthCheckPort: def.PortWaitCheck.Port,
			Label:           def.Label,
		},
		HealthCheckTimeout: time.Duration(def.PortWaitCheck.Timeout) * time.Second,
	}
}

func makeResourceMap(m map[string]*containerDefinition) map[string]*ResourceDefinition {
	if m == nil {
		return nil
	}
	rmap := make(map[string]*ResourceDefinition, len(m))
	for k, cd := range m {
		rmap[k] = cd.toResource()
	}
	return rmap
}

type portWaitCheck struct {
	Port    int `yaml:"port"`
	Timeout int `yaml:"timeout"`
}

// parseEnv fills a map of variables from a list of variables in the
// form "key=value". If the list contains duplicate variables, only the last
// value in the slice for each duplicate key is used.
func parseEnv(dst map[string]EnvTemplate, vars []string) error {
	if len(vars) == 0 {
		return nil
	}
	for _, kv := range vars {
		k, v, err := parseVar(kv)
		if err != nil {
			return err
		}
		dst[k] = EnvTemplate(v)
	}
	return nil
}

func parseVar(kv string) (k, v string, err error) {
	i := strings.IndexByte(kv, '=')
	if i == -1 {
		return "", "", fmt.Errorf("invalid variable %q", kv)
	}
	return kv[:i], kv[i+1:], nil
}
