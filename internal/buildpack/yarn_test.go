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

func TestYarn(t *testing.T) {
	const version = "1.22.10"
	ctx := testlog.WithTB(context.Background(), t)
	// TODO(light): Yarn depends on Node but won't install it by itself.
	yarnContext, _ := testInstall(ctx, t, "node:12.19.0", "yarn:"+version)
	versionOutput := new(strings.Builder)
	err := yarnContext.Run(ctx, &biome.Invocation{
		Argv:   []string{"yarn", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("yarn --version output:\n%s", versionOutput)
	if err != nil {
		t.Errorf("yarn --version: %v", err)
	}
	if got := versionOutput.String(); !strings.Contains(got, version) {
		t.Errorf("yarn --version output does not include %q", version)
	}
}
