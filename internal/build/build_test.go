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

package build

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestExecute(t *testing.T) {
	type commandRecord struct {
		argv []string
		env  biome.Environment
		dir  string
	}
	tests := []struct {
		name      string
		target    *yb.Target
		errorOn   map[string]struct{}
		want      []commandRecord
		wantError bool
	}{
		{
			name: "NoCommands",
			target: &yb.Target{
				Name: yb.DefaultTarget,
			},
			want: nil,
		},
		{
			name: "CommandSequence",
			target: &yb.Target{
				Name: yb.DefaultTarget,
				Commands: []string{
					`echo "Hello, World!"`,
					`cat < foo.txt > bar.txt`, // intentionally using shell-like syntax
				},
			},
			want: []commandRecord{
				{argv: []string{"echo", "Hello, World!"}},
				{argv: []string{"cat", "<", "foo.txt", ">", "bar.txt"}},
			},
		},
		{
			name: "ErrorStopsExecution",
			target: &yb.Target{
				Name: yb.DefaultTarget,
				Commands: []string{
					`echo "Before"`,
					`bork`,
					`echo "After"`,
				},
			},
			errorOn: map[string]struct{}{"bork": {}},
			want: []commandRecord{
				{argv: []string{"echo", "Before"}},
				{argv: []string{"bork"}},
			},
			wantError: true,
		},
		{
			name: "EmptyCommand",
			target: &yb.Target{
				Name: yb.DefaultTarget,
				Commands: []string{
					`echo "Hello, World!"`,
					`   `,
				},
			},
			wantError: true,
		},
		{
			name: "RunDir",
			target: &yb.Target{
				Name:   yb.DefaultTarget,
				RunDir: "foo",
				Commands: []string{
					`echo "Hello, World!"`,
				},
			},
			want: []commandRecord{
				{argv: []string{"echo", "Hello, World!"}, dir: "foo"},
			},
		},
		{
			name: "Chdir",
			target: &yb.Target{
				Name: yb.DefaultTarget,
				Commands: []string{
					`cd foo`,
					`echo "Hello, World!"`,
				},
			},
			want: []commandRecord{
				{argv: []string{"echo", "Hello, World!"}, dir: "foo"},
			},
		},
		{
			name: "Chdir/Empty",
			target: &yb.Target{
				Name: yb.DefaultTarget,
				Commands: []string{
					`echo "Hello, World!"`,
					`cd `,
					`echo "Hello, World!"`,
				},
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testlog.WithTB(context.Background(), t)
			var mu sync.Mutex
			var got []commandRecord
			bio := &biome.Fake{
				Separator: '/',
				RunFunc: func(ctx context.Context, invoke *biome.Invocation) error {
					mu.Lock()
					defer mu.Unlock()
					envCopy := make(map[string]string)
					for k, v := range invoke.Env.Vars {
						envCopy[k] = v
					}
					got = append(got, commandRecord{
						argv: append([]string(nil), invoke.Argv...),
						env: biome.Environment{
							Vars:        envCopy,
							PrependPath: append([]string(nil), invoke.Env.PrependPath...),
							AppendPath:  append([]string(nil), invoke.Env.AppendPath...),
						},
						dir: invoke.Dir,
					})
					if len(invoke.Argv) == 0 {
						return errors.New("len(argv) == 0")
					}
					if _, shouldFail := test.errorOn[invoke.Argv[0]]; shouldFail {
						return errors.New("fault injection!")
					}
					return nil
				},
			}
			err := Execute(ctx, Sys{Biome: bio}, nil, test.target)
			if err != nil {
				if test.wantError {
					t.Logf("Build: %v (expected)", err)
				} else {
					t.Errorf("Build: %v; want <nil>", err)
				}
			} else if err == nil && test.wantError {
				t.Error("Build did not return an error")
			}
			diff := cmp.Diff(test.want, got,
				cmp.AllowUnexported(commandRecord{}),
				cmpopts.EquateEmpty(),
				// Clean the directories before comparing.
				cmp.FilterPath(
					func(path cmp.Path) bool {
						return path.Index(-2).Type() == reflect.TypeOf(commandRecord{}) &&
							path.Last().(cmp.StructField).Name() == "dir"
					},
					cmp.Comparer(func(dir1, dir2 string) bool {
						return biome.CleanPath(bio, dir1) == biome.CleanPath(bio, dir2)
					}),
				),
			)
			if diff != "" {
				t.Errorf("commands (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMain(m *testing.M) {
	testlog.Main(nil)
	os.Exit(m.Run())
}
