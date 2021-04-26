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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourbase/narwhal"
)

const docsURL = "https://docs.yourbase.io"

// DefaultTarget is the name of the target that should be built when no
// arguments are given to yb build.
const DefaultTarget = "default"

// DefaultExecEnvironment is the name of the execution environment variable set
// that should be used when no options are given to yb exec.
const DefaultExecEnvironment = "default"

// PackageConfigFilename is the name of the file at the base of a package
// directory containing the package's configuration.
const PackageConfigFilename = ".yourbase.yml"

// Package is a parsed build configuration (from .yourbase.yml).
type Package struct {
	// Name is the name of the package directory.
	Name string
	// Path is the absolute path to the package directory.
	Path string
	// Targets is the set of targets in the package, keyed by target name.
	Targets map[string]*Target
	// ExecEnvironments is the set of targets representing the exec phase
	// in the configuration, keyed by environment name.
	ExecEnvironments map[string]*Target
}

// LoadPackage loads the package for the given .yourbase.yml file.
func LoadPackage(configPath string) (*Package, error) {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	configYAML, err := ioutil.ReadFile(configPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("load package %s: %w\nTry running in the package directory or creating %s if it is missing. See %s", configPath, err, filepath.Base(configPath), docsURL)
	}
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	pkg, err := parse(filepath.Dir(configPath), configYAML)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	targets := make([]*Target, 0, len(pkg.Targets))
	for _, target := range pkg.Targets {
		targets = append(targets, target)
	}
	if _, err := buildOrder(targets); err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	return pkg, nil
}

// A Target is a buildable unit.
type Target struct {
	Name    string
	Package *Package
	Deps    map[*Target]struct{}
	Tags    map[string]string

	// Container specifies the container environment that should be used to run
	// the commands in if container execution is requested. It will never be nil.
	Container *narwhal.ContainerDefinition
	// UseContainer indicates whether this target requires executing the commands
	// inside a container.
	UseContainer bool

	Commands   []string
	RunDir     string
	Env        map[string]EnvTemplate
	Buildpacks map[string]BuildpackSpec
	Resources  map[string]*ResourceDefinition
}

type ResourceDefinition struct {
	narwhal.ContainerDefinition

	HealthCheckTimeout time.Duration
}

// BuildOrder returns a topological sort of the targets needed to build the
// given target(s). If a single argument is passed, then the last element in the
// returned slice is always the argument.
func BuildOrder(desired ...*Target) []*Target {
	targetList, err := buildOrder(desired)
	if err != nil {
		panic(err)
	}
	return targetList
}

func buildOrder(desired []*Target) ([]*Target, error) {
	type stackFrame struct {
		target *Target
		done   bool
	}
	stk := make([]stackFrame, 0, len(desired))
	for i := len(desired) - 1; i >= 0; i-- {
		stk = append(stk, stackFrame{target: desired[i]})
	}

	var targetList []*Target
	marks := make(map[*Target]int)
	for len(stk) > 0 {
		curr := stk[len(stk)-1]
		stk = stk[:len(stk)-1]

		if curr.done {
			marks[curr.target] = 2
			targetList = append(targetList, curr.target)
			continue
		}
		switch marks[curr.target] {
		case 0:
			// First visit. Revisit once all dependencies have been added to the list.
			marks[curr.target] = 1
			stk = append(stk, stackFrame{target: curr.target, done: true})
			for dep := range curr.target.Deps {
				stk = append(stk, stackFrame{target: dep})
			}
		case 1:
			// Cycle.
			intermediaries := findCycle(curr.target)
			formatted := new(strings.Builder)
			for _, target := range intermediaries {
				formatted.WriteString(target.Name)
				formatted.WriteString(" -> ")
			}
			formatted.WriteString(curr.target.Name)
			return nil, fmt.Errorf("target %s has a cycle: %s", curr.target.Name, formatted)
		}
	}
	return targetList, nil
}

func findCycle(target *Target) []*Target {
	var paths [][]*Target
	for dep := range target.Deps {
		paths = append(paths, []*Target{dep})
	}
	for {
		// Dequeue.
		curr := paths[0]
		copy(paths, paths[1:])
		paths[len(paths)-1] = nil
		paths = paths[:len(paths)-1]

		// Check if the path leads back to the original target.
		deps := curr[len(curr)-1].Deps
		if _, done := deps[target]; done {
			return curr
		}

		// Advance paths.
		for dep := range deps {
			paths = append(paths, append(curr[:len(curr):len(curr)], dep))
		}
	}
}

// BuildpackSpec is a buildpack specifier, consisting of a name and a version.
type BuildpackSpec string

// ParseBuildpackSpec validates a buildpack specifier string.
func ParseBuildpackSpec(s string) (BuildpackSpec, error) {
	i := strings.IndexByte(s, ':')
	if i == -1 {
		return "", fmt.Errorf("parse buildpack %q: no version specified (missing ':')", s)
	}
	return BuildpackSpec(s), nil
}

func (spec BuildpackSpec) Name() string {
	i := strings.IndexByte(string(spec), ':')
	if i == -1 {
		panic("Name() called on invalid spec: " + string(spec))
	}
	return string(spec[:i])
}

func (spec BuildpackSpec) Version() string {
	i := strings.IndexByte(string(spec), ':')
	if i == -1 {
		panic("Version() called on invalid spec: " + string(spec))
	}
	return string(spec[i+1:])
}

// EnvTemplate is an expression for an environment variable value. It's mostly a
// literal string, but may include substitutions for container IP addresses in
// the form `{{ .Containers.IP "mycontainer" }}`.
type EnvTemplate string
