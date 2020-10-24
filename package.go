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

	"gopkg.in/yaml.v2"
)

type Package struct {
	Name     string
	Path     string
	Manifest *BuildManifest
}

func LoadPackage(configPath string) (*Package, error) {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	configYAML, err := ioutil.ReadFile(configPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("load package %s: %w\nTry running in the package directory or creating %s if it is missing. See %s", configPath, err, filepath.Base(configPath), DOCS_URL)
	}
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	manifest := new(BuildManifest)
	if err := yaml.Unmarshal([]byte(configYAML), &manifest); err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	err = mergeDeps(manifest)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", configPath, err)
	}
	dir := filepath.Dir(configPath)
	return &Package{
		Name:     filepath.Base(dir),
		Path:     dir,
		Manifest: manifest,
	}, nil
}

// mergeDeps overrides and merge build dependencies into
// the BuildTarget.Dependencies.Build field. Adding globally defined deps to
// per-build target defined dependencies, where it wasn't added.
func mergeDeps(b *BuildManifest) error {
	globalDepsMap := make(map[string]string) // tool -> version
	globalDeps := b.Dependencies.Build
	targetList := b.BuildTargets

	for _, dep := range globalDeps {
		spec, err := ParseBuildpackSpec(dep)
		if err != nil {
			return fmt.Errorf("merging/overriding build localDeps: %w", err)
		}
		globalDepsMap[spec.Name()] = spec.Version()
	}
	for _, tgt := range targetList {
		tgtToolMap := make(map[string]string)
		for tool, version := range globalDepsMap {
			tgtToolMap[tool] = version
		}
		for _, dep := range tgt.Dependencies.Build {
			spec, err := ParseBuildpackSpec(dep)
			if err != nil {
				return fmt.Errorf("merging/overriding build localDeps: %w", err)
			}
			tgtToolMap[spec.Name()] = spec.Version()
		}
		tgt.Dependencies.Build = tgt.Dependencies.Build[:0]
		for tool, version := range tgtToolMap {
			tgt.Dependencies.Build = append(tgt.Dependencies.Build, tool+":"+version)
		}
	}
	return nil
}
