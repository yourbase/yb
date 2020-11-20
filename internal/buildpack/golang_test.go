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

package buildpack

import (
	"context"
	"strings"
	"testing"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestGo(t *testing.T) {
	const version = "1.15.2"
	ctx := testlog.WithTB(context.Background(), t)
	goBiome, _ := testInstall(ctx, t, "go:"+version)
	versionOutput := new(strings.Builder)
	err := goBiome.Run(ctx, &biome.Invocation{
		Argv:   []string{"go", "version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("go version output:\n%s", versionOutput)
	if err != nil {
		t.Errorf("go version: %v", err)
	}
	if got := versionOutput.String(); !strings.Contains(got, version) {
		t.Errorf("go version output does not include %q", version)
	}

	// Verify that we can run `go install`'d binaries.
	// The local biome does use the ambient PATH, so this may have false
	// positives, but most people don't have stringer installed and it has
	// relatively few dependencies, so it's a good candidate.
	getOutput := new(strings.Builder)
	err = goBiome.Run(ctx, &biome.Invocation{
		Argv:   []string{"go", "get", "golang.org/x/tools/cmd/stringer"},
		Stdout: getOutput,
		Stderr: getOutput,
	})
	t.Logf("go get output:\n%s", getOutput)
	if err != nil {
		t.Fatalf("go get: %v", err)
	}
	stringerOutput := new(strings.Builder)
	err = goBiome.Run(ctx, &biome.Invocation{
		Argv:   []string{"stringer", "-help"},
		Stdout: stringerOutput,
		Stderr: stringerOutput,
	})
	t.Logf("stringer output:\n%s", stringerOutput)
	if err != nil {
		t.Fatalf("stringer: %v", err)
	}
}
