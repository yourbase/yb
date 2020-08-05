package packages

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/yb/types"
)

func TestMergeDeps(t *testing.T) {
	const dummyGoToolSpec = "go:1.14.6"
	tests := []struct {
		name string
		b    *types.BuildManifest
		want *types.BuildManifest
	}{
		{
			name: "Empty",
			b:    &types.BuildManifest{},
			want: &types.BuildManifest{},
		},
		{
			name: "Global",
			b: &types.BuildManifest{
				Dependencies: types.DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []types.BuildTarget{
					{
						Name: "default",
					},
				},
			},
			want: &types.BuildManifest{
				Dependencies: types.DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []types.BuildTarget{
					{
						Name: "default",
						Dependencies: types.BuildDependencies{
							Build: []string{dummyGoToolSpec},
						},
					},
				},
			},
		},
		{
			name: "OverrideVersionLocally",
			b: &types.BuildManifest{
				Dependencies: types.DependencySet{
					Build: []string{"go:1.13"},
				},
				BuildTargets: []types.BuildTarget{
					{
						Name: "default",
						Dependencies: types.BuildDependencies{
							Build: []string{"go:1.14"},
						},
					},
				},
			},
			want: &types.BuildManifest{
				Dependencies: types.DependencySet{
					Build: []string{"go:1.13"},
				},
				BuildTargets: []types.BuildTarget{
					{
						Name: "default",
						Dependencies: types.BuildDependencies{
							Build: []string{"go:1.14"},
						},
					},
				},
			},
		},
		{
			name: "AddNewDepInTarget",
			b: &types.BuildManifest{
				Dependencies: types.DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []types.BuildTarget{
					{
						Name: "default",
						Dependencies: types.BuildDependencies{
							Build: []string{"java:1.8"},
						},
					},
				},
			},
			want: &types.BuildManifest{
				Dependencies: types.DependencySet{
					Build: []string{dummyGoToolSpec},
				},
				BuildTargets: []types.BuildTarget{
					{
						Name: "default",
						Dependencies: types.BuildDependencies{
							Build: []string{"java:1.8", dummyGoToolSpec},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := mergeDeps(test.b)
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
					return path.Index(-2).Type() == reflect.TypeOf(types.BuildDependencies{}) &&
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
