// Copyright 2021 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
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
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBuildOrder(t *testing.T) {
	tests := []struct {
		name       string
		desired    []string
		acceptable [][]string
	}{
		{
			name:    "Empty",
			desired: []string{},
			acceptable: [][]string{
				{},
			},
		},
		{
			name:    "Leaf",
			desired: []string{"default"},
			acceptable: [][]string{
				{"default"},
			},
		},
		{
			name:    "DirectDep",
			desired: []string{"a"},
			acceptable: [][]string{
				{"b", "a"},
			},
		},
		{
			name:    "SharedDep",
			desired: []string{"a", "b"},
			acceptable: [][]string{
				{"c", "b", "a"},
				{"c", "a", "b"},
			},
		},
		{
			name:    "NamedDep",
			desired: []string{"a", "b"},
			acceptable: [][]string{
				{"b", "a"},
			},
		},
		{
			name:    "IndirectDep",
			desired: []string{"a"},
			acceptable: [][]string{
				{"c", "b", "a"},
			},
		},
		{
			name:    "DirectAndIndirectDep",
			desired: []string{"a"},
			acceptable: [][]string{
				{"c", "b", "a"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			configData, err := os.ReadFile(filepath.Join("testdata", "BuildOrder", test.name+".yml"))
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(dir, PackageConfigFilename)
			if err := os.WriteFile(configPath, configData, 0o666); err != nil {
				t.Fatal(err)
			}
			pkg, err := LoadPackage(configPath)
			if err != nil {
				t.Fatal(err)
			}

			desired := make([]*Target, 0, len(test.desired))
			for _, name := range test.desired {
				target := pkg.Targets[name]
				if target == nil {
					t.Fatalf("target %q not found", name)
				}
				desired = append(desired, target)
			}
			got := BuildOrder(desired...)
			gotNames := make([]string, 0, len(got))
			for _, target := range got {
				gotNames = append(gotNames, target.Name)
			}
			for _, wantNames := range test.acceptable {
				if cmp.Equal(wantNames, gotNames) {
					return
				}
			}
			t.Error("Bad ordering:", gotNames)
			t.Log("Wanted one of:")
			for _, wantNames := range test.acceptable {
				t.Log(wantNames)
			}
		})
	}
}
