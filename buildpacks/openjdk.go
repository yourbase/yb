package buildpacks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/johnewart/archiver"
	"github.com/yourbase/yb/plumbing"
	"zombiezen.com/go/log"
)

type javaBuildTool struct {
	version      string
	spec         buildToolSpec
	majorVersion int64
	minorVersion int64
	patchVersion int64
	subVersion   string
}

func newJavaBuildTool(ctx context.Context, toolSpec buildToolSpec) javaBuildTool {

	version := toolSpec.version

	vparts := strings.SplitN(version, "+", 2)
	subVersion := ""
	if len(vparts) > 1 {
		subVersion = vparts[1]
		version = vparts[0]
	}

	parts := strings.Split(version, ".")

	majorVersion, err := convertVersionPiece(parts, 0)
	if err != nil {
		log.Debugf(ctx, "Error when parsing majorVersion %d: %v", majorVersion, err)
	}
	minorVersion, err := convertVersionPiece(parts, 1)
	if err != nil {
		log.Debugf(ctx, "Error when parsing minorVersion %d: %v", minorVersion, err)
	}
	patchVersion, err := convertVersionPiece(parts, 2)
	if err != nil {
		log.Debugf(ctx, "Error when parsing patchVersion %d: %v", patchVersion, err)
	}

	// Sometimes a patchVersion can represent a subVersion
	// e.g.: java:8.252.09 instead of java:8.252+09
	if majorVersion != 11 && majorVersion != 14 && subVersion == "" && patchVersion > 0 && patchVersion < 100 {
		subVersion = fmt.Sprintf("%02d", patchVersion)
	}

	log.Debugf(ctx, "majorVersion: %d, minorVersion: %d, patchVersion: %d, subVersion: %s",
		majorVersion, minorVersion, patchVersion, subVersion)

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
			subVersion = "9"
		case 14:
			subVersion = "36"
		default:
			subVersion = ""
		}
	}

	tool := javaBuildTool{
		version:      toolSpec.version,
		majorVersion: majorVersion,
		minorVersion: minorVersion,
		patchVersion: patchVersion,
		subVersion:   subVersion,
		spec:         toolSpec,
	}

	return tool
}

func (bt javaBuildTool) installDir() string {
	return filepath.Join(bt.spec.cacheDir, "java")
}

func (bt javaBuildTool) javaDir() string {
	opsys := OS()
	// Versions..
	archiveDir := ""
	if bt.majorVersion == 8 {
		archiveDir = fmt.Sprintf("jdk%du%d-b%s", bt.majorVersion, bt.minorVersion, bt.subVersion)
	} else {
		archiveDir = fmt.Sprintf("jdk-%d.%d.%d+%s", bt.majorVersion, bt.minorVersion, bt.patchVersion, bt.subVersion)
	}

	basePath := filepath.Join(bt.installDir(), archiveDir)

	if opsys == "darwin" {
		basePath = filepath.Join(basePath, "Contents", "Home")

	}
	return basePath
}

func convertVersionPiece(parts []string, index int) (piece int64, err error) {
	if len(parts) >= index+1 {
		trimmed := strings.TrimLeft(parts[index], "0")
		piece, err = strconv.ParseInt(trimmed, 0, 64)
		if err != nil {
			err = fmt.Errorf("failed to parse %s: %v", parts[index], err)
		}
	}
	return
}

func (bt javaBuildTool) setup(ctx context.Context) error {
	javaDir := bt.javaDir()
	cmdPath := fmt.Sprintf("%s/bin", javaDir)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	log.Infof(ctx, "Setting PATH to %s", newPath)
	os.Setenv("PATH", newPath)
	log.Infof(ctx, "Setting JAVA_HOME to %s", javaDir)
	os.Setenv("JAVA_HOME", javaDir)

	return nil
}

func (bt javaBuildTool) downloadURL(ctx context.Context) string {

	// This changes pretty dramatically depending on major version :~(
	// Using HotSpot version, we should consider an OpenJ9 option
	urlPattern := ""
	if bt.majorVersion < 9 {
		// TODO add openjdk8 OpenJ9 support
		urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{.MajorVersion}}-binaries/releases/download/jdk{{.MajorVersion}}u{{.MinorVersion}}-b{{.SubVersion}}/OpenJDK{{.MajorVersion}}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{.MajorVersion}}u{{.MinorVersion}}b{{.SubVersion}}.{{.Extension}}"
	} else {
		if bt.majorVersion < 14 {
			// OpenJDK 9 has a whole other scheme
			if bt.majorVersion == 9 && bt.subVersion == "181" {
				urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{ .MajorVersion }}_{{ .SubVersion }}.{{ .Extension }}"
			} else {
				urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}_{{ .SubVersion }}.{{ .Extension }}"
			}
		} else {
			// 14: https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_aarch64_linux_hotspot_14_36.tar.gz
			urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{ .MajorVersion }}_{{ .SubVersion }}.{{ .Extension }}"
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

	log.Debugf(ctx, "URL params: %#v", data)

	url, err := plumbing.TemplateToString(urlPattern, data)

	if err != nil {
		log.Errorf(ctx, "Error generating download URL: %v", err)
	}

	return url
}

func (bt javaBuildTool) install(ctx context.Context) error {
	javaInstallDir := bt.installDir()
	javaPath := bt.javaDir()

	if err := os.MkdirAll(javaInstallDir, 0777); err != nil {
		return fmt.Errorf("install Java: %w", err)
	}

	if _, err := os.Stat(javaPath); err == nil {
		log.Infof(ctx, "Java v%s located in %s!", bt.version, javaPath)
	} else {
		log.Infof(ctx, "Will install Java v%s into %s", bt.version, javaInstallDir)
		downloadURL := bt.downloadURL(ctx)

		log.Infof(ctx, "Downloading from URL %s ", downloadURL)
		localFile, err := plumbing.DownloadFileWithCache(ctx, http.DefaultClient, downloadURL)
		if err != nil {
			log.Errorf(ctx, "Unable to download: %v", err)
			return err
		}
		err = archiver.Unarchive(localFile, javaInstallDir)
		if err != nil {
			log.Errorf(ctx, "Unable to decompress: %v", err)
			return err
		}

	}

	return nil

}
