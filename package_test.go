package yb

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/narwhal"
)

func TestLoadPackage(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Source files are under testdata/LoadPackage.
	tests := []struct {
		name string
		want *Package
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
						Buildpacks: map[string]BuildpackSpec{
							"python": "python:3.7.7",
						},
						Resources: map[string]*narwhal.ContainerDefinition{
							"db": {Image: "yourbase/api_dev_db"},
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
						Buildpacks: map[string]BuildpackSpec{
							"python": "python:3.7.7",
						},
						Resources: map[string]*narwhal.ContainerDefinition{
							"db": {Image: "yourbase/api_dev_db"},
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(workingDir, "testdata", "LoadPackage", filepath.FromSlash(test.name+".yml"))
			got, err := LoadPackage(configPath)
			if err != nil {
				t.Fatal("LoadPackage:", err)
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
