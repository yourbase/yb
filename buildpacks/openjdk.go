package buildpacks

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
)

type JavaBuildTool struct {
	version      string
	spec         BuildToolSpec
	majorVersion int64
	minorVersion int64
	patchVersion int64
	subVersion   string
}

func NewJavaBuildTool(toolSpec BuildToolSpec) JavaBuildTool {

	vparts := strings.SplitN(toolSpec.Version, "+", 2)
	subVersion := ""
	version := toolSpec.Version
	if len(vparts) > 1 {
		subVersion = vparts[1]
		version = vparts[0]
	}

	parts := strings.Split(version, ".")
	c := len(parts)

	var majorVersion int64
	var minorVersion int64
	var patchVersion int64

	if c >= 1 {
		majorVersion, _ = strconv.ParseInt(parts[0], 0, 64)
		if c >= 2 {
			minorVersion, _ = strconv.ParseInt(parts[1], 0, 64)
			if c >= 3 {
				patchVersion, _ = strconv.ParseInt(parts[2], 0, 64)
			}
		}
	}

	// Maybe we just require people format it with the build number?
	// Alternatively we can have a table of defaults somewhere
	if subVersion == "" {
		switch majorVersion {
		case 8:
			subVersion = "08"
		case 9:
			subVersion = "11"
		case 10:
			subVersion = "13.1"
		case 11:
			subVersion = "10"
		case 12:
			subVersion = "10"
		case 13:
			subVersion = "8"
		case 14:
			subVersion = "36"
		default:
			subVersion = ""
		}
	}

	tool := JavaBuildTool{
		version:      toolSpec.Version,
		majorVersion: majorVersion,
		minorVersion: minorVersion,
		patchVersion: patchVersion,
		subVersion:   subVersion,
		spec:         toolSpec,
	}

	return tool
}

func (bt JavaBuildTool) Version() string {
	return bt.version
}

func (bt JavaBuildTool) JavaDir(installDir string) string {
	opsys := OS()
	// Versions..
	archiveDir := ""
	if bt.majorVersion == 8 {
		archiveDir = fmt.Sprintf("jdk%du%d-b%s", bt.majorVersion, bt.minorVersion, bt.subVersion)
	} else {
		archiveDir = fmt.Sprintf("jdk-%d.%d.%d+%s", bt.majorVersion, bt.minorVersion, bt.patchVersion, bt.subVersion)
	}

	basePath := filepath.Join(installDir, archiveDir)

	if opsys == "darwin" {
		basePath = filepath.Join(basePath, "Contents", "Home")
	}

	return basePath
}

func (bt JavaBuildTool) Setup(ctx context.Context, installDir string) error {
	javaDir := bt.JavaDir(installDir)
	t := bt.spec.InstallTarget
	t.SetEnv("JAVA_HOME", javaDir)

	cmdPath := filepath.Join(javaDir, "bin")
	t.PrependToPath(ctx, cmdPath)

	return nil
}

func (bt JavaBuildTool) DownloadURL(ctx context.Context) (string, error) {

	// This changes pretty dramatically depending on major version :~(
	// Using HotSpot version, we should consider an OpenJ9 option
	urlPattern := ""
	if bt.majorVersion < 9 {
		urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{.MajorVersion}}-binaries/releases/download/jdk{{.MajorVersion}}u{{.MinorVersion}}-b{{.SubVersion}}/OpenJDK{{.MajorVersion}}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{.MajorVersion}}u{{.MinorVersion}}b{{.SubVersion}}.{{.Extension}}"
	} else {
		if bt.majorVersion < 14 {
			urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_x64_linux_hotspot_{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}_{{ .SubVersion }}.{{ .Extension }}"
		} else {
			// 14: https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_aarch64_linux_hotspot_14_36.tar.gz
			urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_x64_linux_hotspot_{{ .MajorVersion }}_{{ .SubVersion }}.{{ .Extension }}"
		}
	}

	arch := "x64"
	extension := "tar.gz"

	operatingSystem := OS()
	if operatingSystem == "darwin" {
		operatingSystem = "mac"
	}

	if operatingSystem == "windows" {
		extension = "zip"
	}

	data := struct {
		OS           string
		Arch         string
		MajorVersion int64
		MinorVersion int64
		PatchVersion int64
		SubVersion   string // not always an int, sometimes a float
		Extension    string
	}{
		operatingSystem,
		arch,
		bt.majorVersion,
		bt.minorVersion,
		bt.patchVersion,
		bt.subVersion,
		extension,
	}

	log.Debugf("URL params: %#v", data)

	url, err := TemplateToString(urlPattern, data)
	return url, err
}

func (bt JavaBuildTool) Install(ctx context.Context) (string, error) {
	t := bt.spec.InstallTarget

	javaInstallDir := filepath.Join(t.ToolsDir(ctx), "java")
	javaPath := bt.JavaDir(javaInstallDir)

	t.MkdirAsNeeded(ctx, javaInstallDir)

	if t.PathExists(ctx, javaPath) {
		log.Infof("Java v%s located in %s!", bt.Version(), javaPath)
		return javaPath, nil
	}
	log.Infof("Will install Java v%s into %s", bt.Version(), javaInstallDir)
	downloadURL, err := bt.DownloadURL(ctx)
	if err != nil {
		log.Errorf("Unable to generate download URL: %v", err)
		return "", err
	}

	log.Infof("Downloading from URL %s ", downloadURL)
	localFile, err := t.DownloadFile(ctx, downloadURL)
	if err != nil {
		log.Errorf("Unable to download: %v", err)
		return "", err
	}
	err = t.Unarchive(ctx, localFile, javaInstallDir)
	if err != nil {
		log.Errorf("Unable to decompress: %v", err)
		return "", err
	}

	return javaInstallDir, nil

}
