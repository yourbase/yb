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

import "testing"

var _ Biome = new(Fake)

func TestFakeJoinPath(t *testing.T) {
	tests := []struct {
		elem []string
		sep  rune
		want string
	}{
		{elem: []string{}, sep: '/', want: ""},
		{elem: []string{"", ""}, sep: '/', want: ""},

		{elem: []string{"a", "b", "c"}, sep: '/', want: "a/b/c"},
		{elem: []string{"a", "b/c"}, sep: '/', want: "a/b/c"},
		{elem: []string{"a", ""}, sep: '/', want: "a"},
		{elem: []string{"", "a"}, sep: '/', want: "a"},

		{elem: []string{"a", "b", "c"}, sep: '\\', want: `a\b\c`},
		{elem: []string{"a", "b\\c"}, sep: '\\', want: `a\b\c`},
		{elem: []string{"a", ""}, sep: '\\', want: "a"},
		{elem: []string{"", "a"}, sep: '\\', want: "a"},

		{elem: []string{"a", "b/c"}, sep: '\\', want: `a\b/c`},
	}
	for _, test := range tests {
		got := (&Fake{Separator: test.sep}).JoinPath(test.elem...)
		if got != test.want {
			t.Errorf("(&Fake{Separator: %q}).Join(%q...) = %q; want %q", test.sep, test.elem, got, test.want)
		}
	}
}

func TestFakeCleanPath(t *testing.T) {
	tests := []struct {
		path string
		sep  rune
		want string
	}{
		{path: "", sep: '/', want: "."},
		{path: "a/c", sep: '/', want: "a/c"},
		{path: "a//c", sep: '/', want: "a/c"},
		{path: "a/c/.", sep: '/', want: "a/c"},
		{path: "a/c/b/..", sep: '/', want: "a/c"},
		{path: "/../a/c", sep: '/', want: "/a/c"},
		{path: "/../a/b/../././/c", sep: '/', want: "/a/c"},
		{path: "", sep: '/', want: "."},

		{path: `a\c`, sep: '\\', want: `a\c`},
		{path: `a\\c`, sep: '\\', want: `a\c`},
		{path: `a\c\.`, sep: '\\', want: `a\c`},
		{path: `a\c\b\..`, sep: '\\', want: `a\c`},
		{path: `\..\a\c`, sep: '\\', want: `\a\c`},
		{path: `\..\a\b\..\.\.\\c`, sep: '\\', want: `\a\c`},
		{path: "", sep: '\\', want: "."},

		{path: "a/c", sep: '\\', want: "a/c"},
		{path: "a//c", sep: '\\', want: "a//c"},
		{path: "a/c/.", sep: '\\', want: "a/c/."},
		{path: "a/c/b/..", sep: '\\', want: "a/c/b/.."},
		{path: "/../a/c", sep: '\\', want: "/../a/c"},
		{path: "/../a/b/../././/c", sep: '\\', want: "/../a/b/../././/c"},
	}
	for _, test := range tests {
		got := CleanPath(&Fake{Separator: test.sep}, test.path)
		if got != test.want {
			t.Errorf("CleanPath(&Fake{Separator: %q}, %q) = %q; want %q", test.sep, test.path, got, test.want)
		}
	}
}
