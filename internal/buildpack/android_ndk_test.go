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
	"testing"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestAndroidNDK(t *testing.T) {
	const version = "r21d"
	ctx := testlog.WithTB(context.Background(), t)
	ndkContext, ndkEnv := testInstall(ctx, t, "androidndk:"+version)
	ndkHome := ndkEnv.Vars["ANDROID_NDK_HOME"]
	if ndkHome == "" {
		t.Fatal("ANDROID_NDK_HOME empty")
	}
	if _, err := biome.EvalSymlinks(ctx, ndkContext, ndkContext.JoinPath(ndkHome, "ndk-build")); err != nil {
		t.Error("Find ndk-build:", err)
	}
}
