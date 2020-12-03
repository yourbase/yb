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

package biome

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var _ interface {
	BiomeCloser
	fileWriter
	dirMaker
	symlinkEvaler
} = EnvBiome{}

func TestEnvironmentMerge(t *testing.T) {
	tests := []struct {
		env1, env2, want Environment
	}{
		{
			env1: Environment{},
			env2: Environment{},
			want: Environment{},
		},
		{
			env1: Environment{Vars: map[string]string{"FOO": "BAR"}},
			env2: Environment{},
			want: Environment{Vars: map[string]string{"FOO": "BAR"}},
		},
		{
			env1: Environment{},
			env2: Environment{Vars: map[string]string{"FOO": "BAR"}},
			want: Environment{Vars: map[string]string{"FOO": "BAR"}},
		},
		{
			env1: Environment{Vars: map[string]string{"FOO": "BAR"}},
			env2: Environment{Vars: map[string]string{"FOO": "BAZ"}},
			want: Environment{Vars: map[string]string{"FOO": "BAZ"}},
		},
		{
			env1: Environment{Vars: map[string]string{"FOO": "BAR"}},
			env2: Environment{Vars: map[string]string{"BAZ": "QUUX"}},
			want: Environment{Vars: map[string]string{
				"FOO": "BAR",
				"BAZ": "QUUX",
			}},
		},
	}
	for _, test := range tests {
		got := test.env1.Merge(test.env2)
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Merging:\n\n%v\n\nand:\n\n%v\n\n-want +got:\n%s", test.env1, test.env2, diff)
		}
	}
}

func TestEnvironmentAppend(t *testing.T) {
	tests := []struct {
		name        string
		env         Environment
		defaultPath string
		pathListSep rune
		want        []string
	}{
		{
			name:        "Empty",
			pathListSep: ':',
			want:        nil,
		},
		{
			name: "Basic",
			env: Environment{
				Vars: map[string]string{
					"FOO":  "BAR",
					"QUUX": "FIZZ",
				},
			},
			pathListSep: ':',
			want: []string{
				"FOO=BAR",
				"QUUX=FIZZ",
			},
		},
		{
			name: "WithDefaultPath",
			env: Environment{
				Vars: map[string]string{
					"FOO":  "BAR",
					"QUUX": "FIZZ",
				},
			},
			defaultPath: "/usr/bin:/bin",
			pathListSep: ':',
			want: []string{
				"FOO=BAR",
				"PATH=/usr/bin:/bin",
				"QUUX=FIZZ",
			},
		},
		{
			name: "WithVarsPath",
			env: Environment{
				Vars: map[string]string{
					"PATH": "/usr/local/bin",
				},
			},
			defaultPath: "/usr/bin:/bin",
			pathListSep: ':',
			want: []string{
				"PATH=/usr/local/bin",
			},
		},
		{
			name: "PathAdded",
			env: Environment{
				PrependPath: []string{"/home/example/bin"},
				AppendPath:  []string{"/junk/bin"},
			},
			defaultPath: "/usr/bin:/bin",
			pathListSep: ':',
			want: []string{
				"PATH=/home/example/bin:/usr/bin:/bin:/junk/bin",
			},
		},
		{
			name: "PathAddedToEmptyPath",
			env: Environment{
				PrependPath: []string{"/home/example/bin"},
				AppendPath:  []string{"/junk/bin"},
			},
			pathListSep: ':',
			want: []string{
				"PATH=/home/example/bin:/junk/bin",
			},
		},
		{
			name: "MultiplePathSegmentsAdded",
			env: Environment{
				PrependPath: []string{"/home/example/bin", "/home/example/bin2"},
				AppendPath:  []string{"/junk/bin", "/waste/bin"},
			},
			defaultPath: "/usr/bin:/bin",
			pathListSep: ':',
			want: []string{
				"PATH=/home/example/bin:/home/example/bin2:/usr/bin:/bin:/junk/bin:/waste/bin",
			},
		},
		{
			name: "MultiplePathSegmentsAddedWithSemicolon",
			env: Environment{
				PrependPath: []string{"/home/example/bin", "/home/example/bin2"},
				AppendPath:  []string{"/junk/bin", "/waste/bin"},
			},
			defaultPath: "/usr/bin:/bin",
			pathListSep: ';',
			want: []string{
				"PATH=/home/example/bin;/home/example/bin2;/usr/bin:/bin;/junk/bin;/waste/bin",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.env.appendTo(nil, test.defaultPath, test.pathListSep)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("env.Append(...) (-want +got):\n%s", diff)
			}
		})
	}
}
