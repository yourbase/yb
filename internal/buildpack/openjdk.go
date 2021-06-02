package buildpack

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"net/http"

	"golang.org/x/net/html"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

func installJava(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (_ biome.Environment, err error) {
	installDir := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "java", "openjdk"+spec.Version())
	desc := sys.Biome.Describe()
	home := installDir
	if desc.OS == biome.MacOS && desc.Arch != "arm64" {
		home = sys.Biome.JoinPath(installDir, "Contents", "Home")
	}
	env := biome.Environment{
		Vars: map[string]string{
			"JAVA_HOME": home,
			// Java seems to read /etc/passwd to fill in the user.home JVM property,
			// which isn't correct in our build environments. This is used for Maven's
			// download cache, so it has the incorrect behavior of storing artifacts
			// in /etc/passwd/home/.m2 instead of $HOME/.m2.
			"JAVA_TOOL_OPTIONS": "-Duser.home=" + sys.Biome.Dirs().Home,
		},
		PrependPath: []string{sys.Biome.JoinPath(home, "bin")},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, home); err == nil {
		log.Infof(ctx, "OpenJDK v%s located in %s", spec.Version(), installDir)
		return env, nil
	}

	log.Infof(ctx, "Installing OpenJDK v%s (%s:%s) in %s", spec.Version(), desc.Arch, desc.OS, installDir)
	var downloadURL string
	if desc.Arch == "arm64" && desc.OS == "darwin" {
		downloadURL, err = azulJDKDownloadURL(spec.Version(), desc)
	} else {
		downloadURL, err = javaDownloadURL(spec.Version(), desc)
	}
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, installDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}

func azulJDKDownloadURL(version string, desc *biome.Descriptor) (string, error) {
	fmt.Println("Downloading Azul JDK...")

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

	resp, err := http.Get("https://cdn.azul.com/zulu/bin/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Printf("%d.%d.%d.%d\n", majorVersion, minorVersion, subVersion, patchVersion)
	var arch string
	if desc.Arch == "arm64" {
		arch = "aarch64"
	}
	var os string
	if desc.OS == "darwin" {
		os = "macosx"
	}

	if majorVersion == 8 {
		patchVersion = minorVersion
		minorVersion = 0
	}

	if majorVersion == -1 {
		return "", fmt.Errorf("No major version provided...")
	}
	verMatchStr := fmt.Sprintf("jdk%d", majorVersion)
	if minorVersion > -1 {
		verMatchStr = fmt.Sprintf("%s.%d", verMatchStr, minorVersion)
		if patchVersion > -1 {
			verMatchStr = fmt.Sprintf("%s.%d", verMatchStr, patchVersion)
		}
	}

	osArchStr := fmt.Sprintf("%s_%s.tar.gz", os, arch)
	fmt.Printf("Search: %s - %s\n", verMatchStr, osArchStr)
	matches := make([]string, 0)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if strings.Contains(a.Val, verMatchStr) {
						if strings.Contains(a.Val, osArchStr) {
							if !strings.Contains(a.Val, "-fx-") {
								matches = append(matches, a.Val)
							}
						}
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// https://cdn.azul.com/zulu/bin/zulu11.48.21-ca-jdk11.0.11-macosx_aarch64.tar.gz
	baseUrl := "https://cdn.azul.com/zulu/bin"
	if len(matches) > 1 {
		maxMinorVersion := int64(-1)
		maxPatchVersion := int64(-1)
		for _, m := range matches {
			fmt.Printf(" * %s\n", m)
			parts := strings.Split(m, "-")
			for _, p := range parts {
				if strings.HasPrefix(p, "jdk") {
					p = strings.Replace(p, "jdk", "", 0)
					verBits := strings.Split(p, ".")
					minor, _ := convertVersionPiece(verBits, 1)
					patch, _ := convertVersionPiece(verBits, 2)
					if minor > maxMinorVersion {
						maxMinorVersion = minor
					}
					if patch > maxPatchVersion {
						maxPatchVersion = patch
					}
				}
			}
		}

		jdkVersionStr := fmt.Sprintf("jdk%d.%d.%d", majorVersion, maxMinorVersion, maxPatchVersion)
		for _, ver := range matches {
			if strings.Contains(ver, jdkVersionStr) {
				return fmt.Sprintf("%s/%s", baseUrl, ver), nil
			}
		}
	} else {
		return fmt.Sprintf("%s/%s", baseUrl, matches[0]), nil
	}

	return "", nil
	// https://cdn.azul.com/zulu/bin/zulu11.48.21-ca-jdk11.0.11-macosx_x64.tar.gz
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
	return templateToString(urlPattern, data)
}

func convertVersionPiece(parts []string, index int) (int64, error) {
	if index >= len(parts) {
		return -1, nil
	}
	return strconv.ParseInt(parts[index], 10, 64)
}
