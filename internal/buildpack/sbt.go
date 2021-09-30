// Copyright 2021 YourBase Inc.
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
	"fmt"
	"strings"
	"os"
	"io/ioutil"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installSBT(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (_ biome.Environment, err error) {
	installDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "sbt", spec.Version())
	desc := sys.Biome.Describe()
	parentDir := os.TempDir()
	socketDir, err := ioutil.TempDir(parentDir, "*-ybsbt")
	if err != nil {
		log.Errorf(ctx, "Error creating directory for SBT sockets: %v", err)
	}
	log.Infof(ctx, "SBT will use %s as the socket directory", socketDir)
//	defer os.RemoveAll(socketDir) // clean up

	env := biome.Environment{
		Vars: map[string]string{
			"SBT_GLOBAL_SERVER_DIR": socketDir,
		},
		PrependPath: []string{sys.Biome.JoinPath(installDir)},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, installDir); err == nil {
		log.Infof(ctx, "SBT v%s located in %s", spec.Version(), installDir)
		return env, nil
	}

	log.Infof(ctx, "Installing SBT v%s in %s", spec.Version(), installDir)
	downloadURL, err := sbtDownloadURL(spec.Version(), desc)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, installDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}

func sbtDownloadURL(version string, desc *biome.Descriptor) (string, error) {
	vparts := strings.SplitN(version, "+", 2)
	subVersion := ""
	if len(vparts) > 1 {
		subVersion = vparts[1]
		version = vparts[0]
	}

	parts := strings.Split(version, ".")

	majorVersion, err := convertVersionPiece(parts, 0)
	if err != nil {
		return "", fmt.Errorf("parse jdk version %q: major: %w", version, err)
	}
	minorVersion, err := convertVersionPiece(parts, 1)
	if err != nil {
		return "", fmt.Errorf("parse jdk version %q: minor: %w", version, err)
	}
	patchVersion, err := convertVersionPiece(parts, 2)
	if err != nil {
		return "", fmt.Errorf("parse jdk version %q: patch: %w", version, err)
	}

	urlPattern := "https://github.com/sbt/sbt/releases/download/v{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}/sbt-{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}.tgz"

	var data struct {
		OS           string
		Arch         string
		MajorVersion int64
		MinorVersion int64
		PatchVersion int64
		SubVersion   string // not always an int, sometimes a float
	}
	data.OS = map[string]string{
		biome.Linux: "linux",
		biome.MacOS: "mac",
	}[desc.OS]
	if data.OS == "" {
		return "", fmt.Errorf("unsupported os %s", desc.OS)
	}
	data.MajorVersion = majorVersion
	data.MinorVersion = minorVersion
	data.PatchVersion = patchVersion
	data.SubVersion = subVersion
	return templateToString(urlPattern, data)
}
