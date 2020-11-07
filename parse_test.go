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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

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
