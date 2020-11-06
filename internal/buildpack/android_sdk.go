package buildpack

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log"
)

const latestAndroidVersion = "4333796"

func installAndroidSDK(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (biome.Environment, error) {
	version := spec.Version()
	if version == "latest" {
		version = latestAndroidVersion
	}
	sdkRoot := sys.Biome.JoinPath(sys.Biome.Dirs().Tools, "android", "android-"+version)
	sdkToolsDir := sys.Biome.JoinPath(sdkRoot, "tools")
	env := biome.Environment{
		Vars: map[string]string{
			"ANDROID_SDK_ROOT": sdkRoot,
			"ANDROID_HOME":     sdkRoot,
		},
		PrependPath: []string{
			sys.Biome.JoinPath(sdkRoot, "tools", "bin"),
			sys.Biome.JoinPath(sdkRoot, "tools"),
		},
	}

	// If directory already exists, then use it.
	if _, err := biome.EvalSymlinks(ctx, sys.Biome, sdkToolsDir); err == nil {
		log.Infof(ctx, "Android SDK v%s located in %s", version, sdkRoot)
		return env, nil
	}

	log.Infof(ctx, "Installing Android SDK v%s in %s", version, sdkRoot)
	desc := sys.Biome.Describe()
	const template = "https://dl.google.com/android/repository/sdk-tools-{{.OS}}-{{.Version}}.zip"
	data := struct {
		OS      string
		Version string
	}{
		map[string]string{
			biome.Linux: "linux",
			biome.MacOS: "darwin",
		}[desc.OS],
		version,
	}
	if data.OS == "" {
		return biome.Environment{}, fmt.Errorf("unsupported os %s", desc.OS)
	}
	downloadURL, err := templateToString(template, data)
	if err != nil {
		return biome.Environment{}, err
	}
	if err := extract(ctx, sys, sdkToolsDir, downloadURL, stripTopDirectory); err != nil {
		return biome.Environment{}, err
	}
	if err := writeAndroidAgreements(ctx, sys.Biome, sdkRoot); err != nil {
		return biome.Environment{}, err
	}
	return env, nil
}

func writeAndroidAgreements(ctx context.Context, bio biome.Biome, androidDir string) error {
	licensesDir := bio.JoinPath(androidDir, "licenses")
	err := biome.MkdirAll(ctx, bio, licensesDir)
	if err != nil {
		return fmt.Errorf("write agreement files: %w", err)
	}
	agreements := []struct {
		filename string
		hash     string
	}{
		{"android-googletv-license", "601085b94cd77f0b54ff86406957099ebe79c4d6"},
		{"android-sdk-license", "24333f8a63b6825ea9c5514f83c2829b004d1fee"},
		{"android-sdk-preview-license", "84831b9409646a918e30573bab4c9c91346d8abd"},
		{"google-gdk-license", "33b6a2b64607f11b759f320ef9dff4ae5c47d97a"},
		{"intel-android-extra-license", "d975f751698a77b662f1254ddbeed3901e976f5a"},
		{"mips-android-sysimage-license", "e9acab5b5fbb560a72cfaecce8946896ff6aab9d"},
	}
	for _, a := range agreements {
		dst := bio.JoinPath(licensesDir, a.filename)
		err := biome.WriteFile(ctx, bio, dst, strings.NewReader(a.hash))
		if err != nil {
			return fmt.Errorf("write agreement files: %s: %w", a.filename, err)
		}
	}
	return nil
}
