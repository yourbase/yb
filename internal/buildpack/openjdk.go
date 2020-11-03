package buildpack

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/plumbing"
	"zombiezen.com/go/log"
)

func installJava(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (_ biome.Environment, err error) {
	installDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "java", "openjdk"+spec.Version())
	desc := sys.Biome.Describe()
	home := installDir
	if desc.OS == biome.MacOS {
		home = sys.Biome.JoinPath(installDir, "Contents", "Home")
	}
	env := biome.Environment{
		Vars: map[string]string{
			"JAVA_HOME": home,
		},
		PrependPath: []string{sys.Biome.JoinPath(home, "bin")},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, home); err == nil {
		log.Infof(ctx, "OpenJDK v%s located in %s", spec.Version(), installDir)
		return env, nil
	}

	log.Infof(ctx, "Installing OpenJDK v%s in %s", spec.Version(), installDir)
	downloadURL, err := javaDownloadURL(spec.Version(), desc)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, installDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}

func javaDownloadURL(version string, desc *biome.Descriptor) (string, error) {
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

	// Sometimes a patchVersion can represent a subVersion
	// e.g.: java:8.252.09 instead of java:8.252+09
	if majorVersion != 11 && majorVersion != 14 && subVersion == "" && patchVersion > 0 && patchVersion < 100 {
		subVersion = fmt.Sprintf("%02d", patchVersion)
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
			subVersion = "9"
		case 14:
			subVersion = "36"
		default:
			subVersion = ""
		}
	}

	// This changes pretty dramatically depending on major version :~(
	// Using HotSpot version, we should consider an OpenJ9 option
	var urlPattern string
	if majorVersion < 9 {
		// TODO add openjdk8 OpenJ9 support
		urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{.MajorVersion}}-binaries/releases/download/jdk{{.MajorVersion}}u{{.MinorVersion}}-b{{.SubVersion}}/OpenJDK{{.MajorVersion}}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{.MajorVersion}}u{{.MinorVersion}}b{{.SubVersion}}.tar.gz"
	} else {
		if majorVersion < 14 {
			// OpenJDK 9 has a whole other scheme
			if majorVersion == 9 && subVersion == "181" {
				urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{ .MajorVersion }}_{{ .SubVersion }}.tar.gz"
			} else {
				urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{ .MajorVersion }}.{{ .MinorVersion }}.{{ .PatchVersion }}_{{ .SubVersion }}.tar.gz"
			}
		} else {
			// 14: https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_aarch64_linux_hotspot_14_36.tar.gz
			urlPattern = "https://github.com/AdoptOpenJDK/openjdk{{ .MajorVersion }}-binaries/releases/download/jdk-{{ .MajorVersion }}%2B{{ .SubVersion }}/OpenJDK{{ .MajorVersion }}U-jdk_{{.Arch}}_{{.OS}}_hotspot_{{ .MajorVersion }}_{{ .SubVersion }}.tar.gz"
		}
	}

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
	data.Arch = map[string]string{
		biome.Intel64: "x64",
	}[desc.Arch]
	if data.Arch == "" {
		return "", fmt.Errorf("unsupported architecture %s", desc.Arch)
	}
	data.MajorVersion = majorVersion
	data.MinorVersion = minorVersion
	data.PatchVersion = patchVersion
	data.SubVersion = subVersion
	return plumbing.TemplateToString(urlPattern, data)
}

func convertVersionPiece(parts []string, index int) (int64, error) {
	if index >= len(parts) {
		return 0, nil
	}
	return strconv.ParseInt(parts[index], 10, 64)
}
