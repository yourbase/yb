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
	"runtime"
	"strings"
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

func TestStandardEnv(t *testing.T) {
	stdenv := appendStandardEnv(nil, runtime.GOOS)

	t.Run("TZ", func(t *testing.T) {
		found := false
		for _, e := range stdenv {
			const prefix = "TZ="
			if !strings.HasPrefix(e, prefix) {
				continue
			}
			found = true
			if got, want := e[len(prefix):], "UTC0"; got != want {
				t.Errorf("TZ = %q; want %q", got, want)
			}
		}
		if !found {
			t.Error("TZ not set")
		}
	})

	t.Run("LANG", func(t *testing.T) {
		found := false
		for _, e := range stdenv {
			const prefix = "LANG="
			if !strings.HasPrefix(e, prefix) {
				continue
			}
			found = true
			if got, want1, want2 := e[len(prefix):], "C.UTF-8", "C"; got != want1 && got != want2 {
				t.Errorf("LANG = %q; want %q or %q", got, want1, want2)
			}
		}
		if !found {
			t.Error("LANG not set")
		}
	})

	t.Run("Charmap", func(t *testing.T) {
		// Run locale tool to get character encoding.
		// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/locale.html
		c := exec.Command("locale", "-k", "charmap")
		c.Env = stdenv
		stdout := new(strings.Builder)
		stderr := new(strings.Builder)
		c.Stdout = stdout
		c.Stderr = stderr
		err := c.Run()
		if stderr.Len() > 0 {
			t.Logf("stderr:\n%s", stderr)
		}
		if err != nil {
			t.Error("locale:", err)
		}
		got := parseLocaleOutput(stdout.String())["charmap"]
		const want = "UTF-8"
		if got != want {
			t.Errorf("charmap = %q; want %q", got, want)
		}
	})
}

func parseLocaleOutput(out string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		eq := strings.Index(line, "=")
		if eq == -1 {
			continue
		}
		k, v := line[:eq], line[eq+1:]
		if strings.HasPrefix(v, `"`) {
			v = strings.TrimPrefix(v, `"`)
			v = strings.TrimSuffix(v, `"`)
		}
		m[k] = v
	}
	return m
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
