package buildpack

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestFlutter(t *testing.T) {
	const version = "1.22.2"
	ctx := testlog.WithTB(context.Background(), t)
	flutterCtx, _ := testInstall(ctx, t, "flutter:"+version)
	versionOutput := new(strings.Builder)
	err := flutterCtx.Run(ctx, &biome.Invocation{
		Argv:   []string{"flutter", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("flutter --version:\n%s", versionOutput)
	if err != nil {
		t.Errorf("flutter --version: %v", err)
	}
	if got := versionOutput.String(); !strings.Contains(got, version) {
		t.Errorf("flutter --version output does not include %q", version)
	}
}

func TestFlutterDownloadURL(t *testing.T) {
	tests := []struct {
		version string
		os      string
		want    string
	}{
		{
			version: "1.17.5",
			os:      biome.Linux,
			want:    "https://storage.googleapis.com/flutter_infra/releases/stable/linux/flutter_linux_1.17.5-stable.tar.xz",
		},
		{
			version: "1.17.0-3.2.pre-beta",
			os:      biome.Linux,
			want:    "https://storage.googleapis.com/flutter_infra/releases/beta/linux/flutter_linux_1.17.0-3.2.pre-beta.tar.xz",
		},
		{
			version: "1.19.0-4.2.pre-beta",
			os:      biome.Linux,
			want:    "https://storage.googleapis.com/flutter_infra/releases/beta/linux/flutter_linux_1.19.0-4.2.pre-beta.tar.xz",
		},
		{
			version: "1.12.13+hotfix.9",
			os:      biome.Linux,
			want:    "https://storage.googleapis.com/flutter_infra/releases/stable/linux/flutter_linux_v1.12.13+hotfix.9-stable.tar.xz",
		},
		{
			version: "1.20.2",
			os:      biome.Linux,
			want:    "https://storage.googleapis.com/flutter_infra/releases/stable/linux/flutter_linux_1.20.2-stable.tar.xz",
		},
		{
			version: "1.17.0-dev.0.0-dev",
			os:      biome.Linux,
			want:    "https://storage.googleapis.com/flutter_infra/releases/dev/linux/flutter_linux_1.17.0-dev.0.0-dev.tar.xz",
		},
	}
	for _, test := range tests {
		desc := &biome.Descriptor{OS: test.os, Arch: biome.Intel64}
		got, err := flutterDownloadURL(test.version, desc)
		if got != test.want || err != nil {
			t.Errorf("flutterDownloadURL(%q, %+v) = %q, %v; want %q, <nil>", test.version, desc, got, err, test.want)
		}
	}

	t.Run("Existence", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping due to -short")
		}
		for _, test := range tests {
			verifyURLExists(t, http.MethodHead, test.want)
		}
	})
}
