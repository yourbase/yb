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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestSetup(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	var gotEnv biome.Environment
	bio := &biome.Fake{
		Separator: '/',
		Descriptor: biome.Descriptor{
			OS:   biome.Linux,
			Arch: biome.Intel64,
		},
		RunFunc: func(_ context.Context, invoke *biome.Invocation) error {
			gotEnv = invoke.Env
			return nil
		},
	}
	// Should not require Docker: no containers in dependencies.
	sys := Sys{Biome: bio}
	gotBiome, err := Setup(ctx, sys, &PhaseDeps{
		TargetName: "default",
		EnvironmentTemplate: map[string]string{
			"FOO": "BAR",
		},
	})
	if err != nil {
		t.Fatal("Setup:", err)
	}
	defer func() {
		if err := gotBiome.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()
	err = gotBiome.Run(ctx, &biome.Invocation{
		Argv: []string{"env"},
	})
	if err != nil {
		t.Error("Run:", err)
	}
	wantEnv := biome.Environment{
		Vars: map[string]string{
			"FOO": "BAR",
		},
	}
	if diff := cmp.Diff(wantEnv, gotEnv, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("invoked environment (-want +got):\n%s", diff)
	}
}

func TestConfigExpansion(t *testing.T) {
	tests := []struct {
		name      string
		exp       configExpansion
		value     string
		want      string
		wantError bool
	}{
		{
			name:  "NoExpansion",
			value: "foo",
			want:  "foo",
		},
		{
			name: "ExpandContainerIP",
			exp: configExpansion{
				Containers: containersExpansion{
					ips: map[string]string{
						"postgres": "12.34.56.78",
					},
				},
			},
			value: `{{ .Containers.IP "postgres" }}`,
			want:  "12.34.56.78",
		},
		{
			name: "ExpandUnknownContainerIP",
			exp: configExpansion{
				Containers: containersExpansion{
					ips: map[string]string{
						"postgres": "12.34.56.78",
					},
				},
			},
			value:     `{{ .Containers.IP "foo" }}`,
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.exp.expand(test.value)
			if err != nil {
				t.Logf("exp.expand(%q) = _, %v", test.value, err)
				if !test.wantError {
					t.Fail()
				}
				return
			}
			if got != test.want || test.wantError {
				errString := "<nil>"
				if test.wantError {
					errString = "<error>"
				}
				t.Errorf("exp.expand(%q) = %q, %v; want %q, %s", test.value, got, err, test.want, errString)
			}
		})
	}
}
