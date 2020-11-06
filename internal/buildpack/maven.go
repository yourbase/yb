package buildpack

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installMaven(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	version := spec.Version()
	dotIndex := strings.IndexByte(version, '.')
	if dotIndex == -1 {
		return biome.Environment{}, fmt.Errorf("invalid version %q", version)
	}
	majorVersion := version[:dotIndex]

	mavenDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "maven", "apache-maven-"+version)
	env := biome.Environment{
		PrependPath: []string{
			sys.Biome.JoinPath(mavenDir, "bin"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, mavenDir); err == nil {
		log.Infof(ctx, "Maven v%s located in %s", spec.Version(), mavenDir)
		return env, nil
	}

	log.Infof(ctx, "Installing Maven v%s in %s", spec.Version(), mavenDir)
	const template = "https://archive.apache.org/dist/maven/maven-{{.MajorVersion}}/{{.Version}}/binaries/apache-maven-{{.Version}}-bin.tar.gz"
	data := struct {
		Version      string
		MajorVersion string
	}{
		Version:      spec.Version(),
		MajorVersion: majorVersion,
	}
	downloadURL, err := templateToString(template, data)
	if dotIndex == -1 {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, mavenDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}
