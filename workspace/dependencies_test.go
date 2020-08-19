package workspace

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestMergeDeps(t *testing.T) {
	const dummyGoToolSpec = "go:1.14.6"
	tests := []struct {
		name string
		b    *BuildManifest
		want *BuildManifest
	}{
		{
			name: "Global",
			b: &BuildManifest{
				Dependencies: DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []BuildTarget{
					{
						Name: "default",
					},
				},
			},
			want: &BuildManifest{
				Dependencies: DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []BuildTarget{
					{
						Name: "default",
						Dependencies: BuildDependencies{
							Build: []string{dummyGoToolSpec},
						},
					},
				},
			},
		},
		{
			name: "OverrideVersionLocally",
			b: &BuildManifest{
				Dependencies: DependencySet{
					Build: []string{"go:1.13"},
				},
				BuildTargets: []BuildTarget{
					{
						Name: "default",
						Dependencies: BuildDependencies{
							Build: []string{"go:1.14"},
						},
					},
				},
			},
			want: &BuildManifest{
				Dependencies: DependencySet{
					Build: []string{"go:1.13"},
				},
				BuildTargets: []BuildTarget{
					{
						Name: "default",
						Dependencies: BuildDependencies{
							Build: []string{"go:1.14"},
						},
					},
				},
			},
		},
		{
			name: "AddNewDepInTarget",
			b: &BuildManifest{
				Dependencies: DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []BuildTarget{
					{
						Name: "default",
						Dependencies: BuildDependencies{
							Build: []string{"java:1.8"},
						},
					},
				},
			},
			want: &BuildManifest{
				Dependencies: DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []BuildTarget{
					{
						Name: "default",
						Dependencies: BuildDependencies{
							Build: []string{"java:1.8", dummyGoToolSpec},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tgts := test.b.BuildTargets
			err := tgts[0].mergeDeps(test.b.Dependencies.Build)
			if err != nil {
				t.Fatal(err)
			}
			diff := cmp.Diff(test.want, test.b,
				cmpopts.EquateEmpty(),
				// Ignore order of BuildDependencies.Build field
				cmp.FilterPath(func(path cmp.Path) bool {
					f, ok := path.Last().(cmp.StructField)
					if !ok {
						return false
					}
					return path.Index(-2).Type() == reflect.TypeOf(BuildDependencies{}) &&
						f.Name() == "Build"
				}, cmpopts.SortSlices(func(s1, s2 string) bool {
					return s1 < s2
				})),
			)
			if diff != "" {
				t.Errorf("manifest (-want +got):\n%s", diff)
			}
		})
	}
}