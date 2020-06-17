package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

//https://archive.apache.org/dist/maven/maven-3/3.3.3/binaries/apache-maven-3.3.3-bin.tar.gz
var MAVEN_DIST_MIRROR = "https://archive.apache.org/dist/maven/"

type MavenBuildTool struct {
	BuildTool
	version string
	spec    BuildToolSpec
}

func NewMavenBuildTool(toolSpec BuildToolSpec) MavenBuildTool {
	tool := MavenBuildTool{
		version: toolSpec.Version,
		spec:    toolSpec,
	}

	return tool
}

func (bt MavenBuildTool) ArchiveFile() string {
	return fmt.Sprintf("apache-maven-%s-bin.tar.gz", bt.Version())
}

func (bt MavenBuildTool) DownloadUrl() string {
	return fmt.Sprintf(
		"%s/maven-%s/%s/binaries/%s",
		MAVEN_DIST_MIRROR,
		bt.MajorVersion(),
		bt.Version(),
		bt.ArchiveFile(),
	)
}

func (bt MavenBuildTool) MajorVersion() string {
	parts := strings.Split(bt.version, ".")
	return parts[0]
}

func (bt MavenBuildTool) Version() string {
	return bt.version
}

func (bt MavenBuildTool) Setup(ctx context.Context, mavenDir string) error {
	t := bt.spec.InstallTarget

	t.PrependToPath(ctx, filepath.Join(mavenDir, "bin"))

	return nil
}

func (bt MavenBuildTool) Install(ctx context.Context) (error, string) {
	t := bt.spec.InstallTarget

	installDir := filepath.Join(t.ToolsDir(ctx), "maven")
	mavenDir := filepath.Join(installDir, "apache-maven-"+bt.Version())

	if t.PathExists(ctx, mavenDir) {
		log.Infof("Maven v%s located in %s!", bt.Version(), mavenDir)
	} else {
		log.Infof("Will install Maven v%s into %s", bt.Version(), installDir)
		downloadUrl := bt.DownloadUrl()

		log.Infof("Downloading Maven from URL %s...", downloadUrl)
		localFile, err := t.DownloadFile(ctx, downloadUrl)
		if err != nil {
			log.Errorf("Unable to download: %v", err)
			return err, ""
		}
		err = t.Unarchive(ctx, localFile, installDir)
		if err != nil {
			log.Errorf("Unable to decompress: %v", err)
			return err, ""
		}

	}

	return nil, mavenDir
}
