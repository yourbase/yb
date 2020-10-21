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

func TestAndroidSDK(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	// TODO(light): The Android SDK depends on very specific versions of Java
	// and won't install them by itself.
	androidContext, _ := testInstall(ctx, t, "java:8.265+01", "android:"+latestAndroidVersion)
	installOutput := new(strings.Builder)
	// TODO(light): There isn't a great "get current version" command AFAICT.
	// I've found that this is typically the first command that gets run by users.
	err := androidContext.Run(ctx, &biome.Invocation{
		Argv:   []string{"sdkmanager", "--install", "tools"},
		Stdout: installOutput,
		Stderr: installOutput,
	})
	t.Logf("sdkmanager output:\n%s", installOutput)
	if err != nil {
		t.Errorf("sdkmanager: %v", err)
	}
}
