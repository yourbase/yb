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

func TestPython(t *testing.T) {
	const version = "3.7.7"
	ctx := testlog.WithTB(context.Background(), t)
	pyBiome, _ := testInstall(ctx, t, "python:"+version)
	versionOutput := new(strings.Builder)
	err := pyBiome.Run(ctx, &biome.Invocation{
		Argv:   []string{"python", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("python --version output:\n%s", versionOutput)
	if err != nil {
		t.Errorf("python --version: %v", err)
	}
	if got := versionOutput.String(); !strings.Contains(got, version) {
		t.Errorf("python --version output does not include %q", version)
	}
}
