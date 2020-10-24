package buildpack

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

func installFlutter(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (_ biome.Environment, err error) {
	dir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "flutter", "flutter-"+spec.Version())
	env := biome.Environment{
		PrependPath: []string{sys.Biome.JoinPath(dir, "bin")},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, dir); err == nil {
		log.Infof(ctx, "Flutter v%s located in %s", spec.Version(), dir)
		return env, nil
	}

	log.Infof(ctx, "Installing Flutter v%s in %s", spec.Version(), dir)
	downloadURL, err := flutterDownloadURL(spec.Version(), sys.Biome.Describe())
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, dir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}

func flutterDownloadURL(version string, desc *biome.Descriptor) (string, error) {
	const template = "https://storage.googleapis.com/flutter_infra/releases/{{.Channel}}/{{.OS}}/flutter_{{.OS}}_{{.Version}}-{{.Channel}}.{{.Extension}}"
	var data struct {
		Channel   string
		OS        string
		Version   string
		Extension string
	}
	switch desc.OS {
	case biome.Linux:
		data.OS = "linux"
		data.Extension = "tar.xz"
	case biome.MacOS:
		data.OS = "macos"
		data.Extension = "zip"
	default:
		return "", fmt.Errorf("unsupported os %s", desc.OS)
	}

	// Starting with flutter 1.17 the version format changed.
	// Adding support for pre version 1.17 with "v" and keep support for no "v"
	// - Pre 1.17 version =>  vx.xx.x or vx.xx.x+hotfix.y
	//   https://storage.googleapis.com/.../flutter_windows_v1.12.13+hotfix.9-stable.zip
	// - 1.17 (and greater?) => 1.17.0 (no "v" in download URL)
	//   https://storage.googleapis.com/.../flutter_windows_1.17.0-stable.zip)
	//
	// Also, yb tacks on a v for customers when we build the URL.
	// This function will be backward compatible (tack on "v"), it will support pre 1.17
	// version with a "v", and support 1.17 and greater
	//
	// Note: We are predicting the future since they could require a "v" again if 1.17.0
	// was a mistake
	const version1170 = "v1.17.0"
	compVersion := version

	// Semver package requires the version to start with "v"
	if !strings.HasPrefix(compVersion, "v") {
		compVersion = "v" + compVersion
	}

	// Below 1.17.0 need the "v", >= to 1.17.0, remove the "v"
	if semver.Compare(compVersion, version1170) < 0 {
		version = compVersion // Need the "v"
	} else {
		version = strings.TrimPrefix(compVersion, "v")
	}
	if strings.Contains(version, "pre") || strings.Contains(version, "dev") {
		// Beta/dev versions are considered to be newer, even if semver sees it differently
		// Those versions started to pop up after v1.17.0: https://medium.com/flutter/flutter-spring-2020-update-f723d898d7af
		version = strings.TrimPrefix(compVersion, "v")
	}

	data.Version = version
	switch {
	case strings.HasSuffix(version, "-beta"):
		data.Version = data.Version[:len(data.Version)-len("-beta")]
		data.Channel = "beta"
	case strings.HasSuffix(version, "-dev"):
		data.Version = data.Version[:len(data.Version)-len("-dev")]
		data.Channel = "dev"
	default:
		data.Channel = "stable"
	}

	return plumbing.TemplateToString(template, data)
}
