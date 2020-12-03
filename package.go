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

	"github.com/yourbase/narwhal"
)

const docsURL = "https://docs.yourbase.io"

// DefaultTarget is the name of the target that should be built when no
// arguments are given to yb build.
const DefaultTarget = "default"

// DefaultExecEnvironment is the name of the execution environment variable set
// that should be used when no options are given to yb exec.
const DefaultExecEnvironment = "default"

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
	pkg, err := parse(configYAML)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	// TODO(light): Validate the package for dependency cycles.
	dir := filepath.Dir(configPath)
	pkg.Name = filepath.Base(dir)
	pkg.Path = dir
	return pkg, nil
}

// A Target is a buildable unit.
type Target struct {
	Name    string
	Package *Package
	Deps    map[*Target]struct{}
	Tags    map[string]string

	Commands   []string
	RunDir     string
	Container  *narwhal.ContainerDefinition
	Env        map[string]EnvTemplate
	Buildpacks map[string]BuildpackSpec
	Resources  map[string]*narwhal.ContainerDefinition
	HostOnly   bool
}

// BuildOrder returns a topological sort of the targets needed to build the
// given target. The last element in the slice is always the argument.
func BuildOrder(desired *Target) []*Target {
	// TODO(ch2750): This only handles direct dependencies.
	// TODO(light): This only handles one target.
	var targetList []*Target
	for dep := range desired.Deps {
		targetList = append(targetList, dep)
	}
	targetList = append(targetList, desired)
	return targetList
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
