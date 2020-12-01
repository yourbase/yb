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
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"zombiezen.com/go/log/testlog"
)

var (
	_ interface {
		BiomeCloser
		fileWriter
		dirMaker
		symlinkEvaler
	} = Local{}

	_ interface {
		BiomeCloser
		fileWriter
		dirMaker
		symlinkEvaler
	} = ExecPrefix{}
)

func TestLocal(t *testing.T) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("Cannot find true:", err)
	}
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("Cannot find false:", err)
	}

	trueExe, err := ioutil.ReadFile(truePath)
	if err != nil {
		t.Fatal(err)
	}
	packageDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(packageDir, "foo"), 0777); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(packageDir, "foo", "xyzzy"), trueExe, 0777); err != nil {
		t.Fatal(err)
	}
	homeDir := t.TempDir()

	tests := []struct {
		name    string
		invoke  *Invocation
		wantErr bool
	}{
		{
			name: "EmptyArgv",
			invoke: &Invocation{
				Argv: []string{},
			},
			wantErr: true,
		},
		{
			name: "AbsPathExe/Success",
			invoke: &Invocation{
				Argv: []string{truePath},
			},
			wantErr: false,
		},
		{
			name: "AbsPathExe/Failure",
			invoke: &Invocation{
				Argv: []string{falsePath},
			},
			wantErr: true,
		},
		{
			name: "JustNameExe/Success",
			invoke: &Invocation{
				Argv: []string{"true"},
			},
			wantErr: false,
		},
		{
			name: "JustNameExe/Failure",
			invoke: &Invocation{
				Argv: []string{"false"},
			},
			wantErr: true,
		},
		{
			name: "RelativeExe",
			invoke: &Invocation{
				// Not using filepath.Join because it cleans away the "./".
				Argv: []string{filepath.FromSlash("./foo/xyzzy")},
			},
			wantErr: false,
		},
		{
			name: "RelativeSubdirExe",
			invoke: &Invocation{
				// Not using filepath.Join because it cleans away the "./".
				Argv: []string{filepath.FromSlash("./xyzzy")},
				Dir:  "foo",
			},
			wantErr: false,
		},
		{
			name: "RelativePATH",
			invoke: &Invocation{
				Argv: []string{"xyzzy"},
				Env: Environment{
					AppendPath: []string{"foo"},
				},
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testlog.WithTB(context.Background(), t)
			l := Local{
				PackageDir: packageDir,
				HomeDir:    homeDir,
			}
			if err := l.Run(ctx, test.invoke); err != nil {
				t.Log("Run:", err)
				if !test.wantErr {
					t.Fail()
				}
				return
			}
			if test.wantErr {
				t.Error("Run succeeded")
			}
		})
	}
}

func TestExecPrefix(t *testing.T) {
	tests := []struct {
		name        string
		prependArgv []string
		argv        []string
		want        []string
	}{
		{
			name: "NoPrefix",
			argv: []string{"echo", "Hello, World!"},
			want: []string{"echo", "Hello, World!"},
		},
		{
			name:        "WithPrefix",
			prependArgv: []string{"sudo", "--"},
			argv:        []string{"echo", "Hello, World!"},
			want:        []string{"sudo", "--", "echo", "Hello, World!"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			bio := ExecPrefix{
				Biome: &Fake{
					Separator: '/',
					RunFunc: func(_ context.Context, invoke *Invocation) error {
						got = append([]string(nil), invoke.Argv...)
						return nil
					},
				},
				PrependArgv: test.prependArgv,
			}
			argv := append([]string(nil), test.argv...)
			err := bio.Run(context.Background(), &Invocation{
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

func TestMain(m *testing.M) {
	testlog.Main(nil)
	os.Exit(m.Run())
}
