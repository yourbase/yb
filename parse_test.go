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
	"path/filepath"
	"reflect"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/narwhal"
)

func TestLoadPackage(t *testing.T) {
	packageDir, err := filepath.Abs(filepath.Join("testdata", "LoadPackage"))
	if err != nil {
		t.Fatal(err)
	}
	// Source files are under testdata/LoadPackage.
	tests := []struct {
		name      string
		want      *Package
		wantError bool
	}{
		{
			name: "Empty",
			want: &Package{},
		},
		{
			name: "TargetDeps",
			want: &Package{
				Targets: map[string]*Target{
					"foo": {
						Name: "foo",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
					},
					"bar": {
						Name: "bar",
						Deps: map[*Target]struct{}{
							{Name: "foo"}: {},
						},
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
					},
				},
			},
		},
		{
			name: "DefaultTarget",
			want: &Package{
				Targets: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
						Commands: []string{
							"/bin/true",
						},
					},
				},
			},
		},
		{
			name: "GlobalDeps",
			want: &Package{
				Targets: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
						Buildpacks: map[string]BuildpackSpec{
							"go": "go:1.14.1",
						},
					},
				},
			},
		},
		{
			name: "OverrideVersionLocally",
			want: &Package{
				Targets: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
						Buildpacks: map[string]BuildpackSpec{
							"go": "go:1.14.1",
						},
					},
				},
			},
		},
		{
			name: "AddNewDepInTarget",
			want: &Package{
				Targets: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
						Buildpacks: map[string]BuildpackSpec{
							"go":   "go:1.14.1",
							"java": "java:1.8",
						},
					},
				},
			},
		},
		{
			name: "Mounts",
			want: &Package{
				Targets: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
							Mounts: []docker.HostMount{
								{
									Source: filepath.Join(packageDir, "relative"),
									Target: "/foo",
									Type:   "bind",
								},
								{
									Source: "/absolute",
									Target: "/bar",
									Type:   "bind",
								},
							},
						},
						UseContainer: true,
						Commands: []string{
							"/bin/true",
						},
					},
				},
			},
		},
		{
			name: "Exec",
			want: &Package{
				ExecEnvironments: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
							Ports: []string{
								"5000",
								"5001",
							},
						},
						UseContainer: true,
						Buildpacks: map[string]BuildpackSpec{
							"python": "python:3.7.7",
						},
						Resources: map[string]*ResourceDefinition{
							"db": {ContainerDefinition: narwhal.ContainerDefinition{
								Image: "yourbase/api_dev_db",
							}},
						},
						Env: map[string]EnvTemplate{
							"DATABASE_URL":   `postgres://yourbase:yourbase@{{ .Containers.IP "db" }}/yourbase`,
							"FLASK_DEBUG":    "1",
							"YB_ENVIRONMENT": "development",
						},
						Commands: []string{
							"honcho start",
						},
					},
					"staging": {
						Name: "staging",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
							Ports: []string{
								"5000",
								"5001",
							},
						},
						UseContainer: true,
						Buildpacks: map[string]BuildpackSpec{
							"python": "python:3.7.7",
						},
						Resources: map[string]*ResourceDefinition{
							"db": {ContainerDefinition: narwhal.ContainerDefinition{
								Image: "yourbase/api_dev_db",
							}},
						},
						Env: map[string]EnvTemplate{
							"DATABASE_URL":   `postgres://yourbase:yourbase@{{ .Containers.IP "db" }}/yourbase`,
							"FLASK_DEBUG":    "1",
							"YB_ENVIRONMENT": "staging",
						},
						Commands: []string{
							"honcho start",
						},
					},
				},
			},
		},
		{
			name: "ExecEmptyDefault",
			want: &Package{
				ExecEnvironments: map[string]*Target{
					"default": {
						Name: "default",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
						Buildpacks: map[string]BuildpackSpec{
							"python": "python:3.7.7",
						},
						Commands: []string{
							"honcho start",
						},
					},
					"staging": {
						Name: "staging",
						Container: &narwhal.ContainerDefinition{
							Image: DefaultContainerImage,
						},
						Buildpacks: map[string]BuildpackSpec{
							"python": "python:3.7.7",
						},
						Env: map[string]EnvTemplate{
							"YB_ENVIRONMENT": "staging",
						},
						Commands: []string{
							"honcho start",
						},
					},
				},
			},
		},
		{
			name:      "Cycle",
			wantError: true,
		},
		{
			name:      "SelfCycle",
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(packageDir, filepath.FromSlash(test.name+".yml"))
			got, err := LoadPackage(configPath)
			if err != nil {
				t.Log("LoadPackage:", err)
				if !test.wantError {
					t.Fail()
				}
				return
			}
			if test.wantError {
				t.Fatal("LoadPackage did not return an error as expected")
			}
			if want := filepath.Dir(configPath); got.Path != want {
				t.Errorf("pkg.Path = %q; want %q", got.Path, want)
			}
			for name, tgt := range got.Targets {
				if tgt.Package != got {
					t.Errorf("pkg.Targets[%q].Package = %p; want %p", name, tgt.Package, got)
				}
			}
			for name, tgt := range got.ExecEnvironments {
				if tgt.Package != got {
					t.Errorf("pkg.ExecEnvironments[%q].Package = %p; want %p", name, tgt.Package, got)
				}
			}
			diff := cmp.Diff(test.want, got,
				cmp.FilterPath(func(p cmp.Path) bool {
					return p.Last().Type() != reflect.TypeOf(map[*Target]struct{}(nil))
				}, cmpopts.EquateEmpty()),
				cmpopts.IgnoreFields(Package{}, "Name", "Path"),
				cmpopts.IgnoreFields(Target{}, "Package"),
				// Compare Deps by name.
				cmp.Comparer(func(set1, set2 map[*Target]struct{}) bool {
					names1 := make(map[string]struct{})
					for tgt := range set1 {
						names1[tgt.Name] = struct{}{}
					}
					names2 := make(map[string]struct{})
					for tgt := range set2 {
						names2[tgt.Name] = struct{}{}
					}
					return cmp.Equal(names1, names2)
				}),
			)
			if diff != "" {
				t.Errorf("package (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseEnv(t *testing.T) {
	tests := []struct {
		vars      []string
		want      map[string]EnvTemplate
		wantError bool
	}{
		{
			vars: nil,
			want: nil,
		},
		{
			vars: []string{"FOO=BAR"},
			want: map[string]EnvTemplate{"FOO": "BAR"},
		},
		{
			vars: []string{"FOO=BAR", "BAZ=QUUX"},
			want: map[string]EnvTemplate{"FOO": "BAR", "BAZ": "QUUX"},
		},
		{
			vars: []string{"FOO=BAR", "FOO=BAZ"},
			want: map[string]EnvTemplate{"FOO": "BAZ"},
		},
		{
			vars:      []string{"FOO"},
			wantError: true,
		},
	}
	for _, test := range tests {
		got := make(map[string]EnvTemplate)
		err := parseEnv(got, test.vars)
		if err != nil {
			t.Logf("parseEnv(m, %q) = %v", test.vars, err)
			if !test.wantError {
				t.Fail()
			}
			continue
		}
		if test.wantError {
			t.Errorf("after parseEnv(m, %q), m = %v; want error", test.vars, got)
		}
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("map (-want +got):\n%s", diff)
		}
	}
}
