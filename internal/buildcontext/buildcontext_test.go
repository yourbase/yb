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

package buildcontext

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	_ Context = Local{}
	_ Context = ExecPrefix{}
)

func TestExecPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix []string
		argv   []string
		want   []string
	}{
		{
			name: "NoPrefix",
			argv: []string{"echo", "Hello, World!"},
			want: []string{"echo", "Hello, World!"},
		},
		{
			name:   "NoPrefix",
			prefix: []string{"skipper", "--"},
			argv:   []string{"echo", "Hello, World!"},
			want:   []string{"skipper", "--", "echo", "Hello, World!"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			bctx := ExecPrefix{
				Context: &Fake{
					Separator: '/',
					RunFunc: func(_ context.Context, invoke *Invocation) error {
						got = append([]string(nil), invoke.Argv...)
						return nil
					},
				},
				Prefix: test.prefix,
			}
			argv := append([]string(nil), test.argv...)
			err := bctx.Run(context.Background(), &Invocation{
				Argv: argv,
			})
			if err != nil {
				t.Error(err)
			}
			if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Argv (-want +got):\n%s", diff)
			}
			if !cmp.Equal(test.argv, argv) {
				t.Error("ExecPrefix mutated input argv")
			}
		})
	}
}
