package buildpack

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/yourbase/yb/internal/biome"
	"zombiezen.com/go/log/testlog"
)

func TestJava(t *testing.T) {
	const majorVersion = "15"
	const version = "15+36"
	ctx := testlog.WithTB(context.Background(), t)
	javaCtx, _ := testInstall(ctx, t, "java:"+version)

	versionOutput := new(strings.Builder)
	err := javaCtx.Run(ctx, &biome.Invocation{
		Argv:   []string{"java", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("java --version:\n%s", versionOutput)
	if err != nil {
		t.Errorf("java --version: %v", err)
	}
	if got := versionOutput.String(); !strings.Contains(got, version) {
		t.Errorf("java --version output does not include %q", version)
	}

	versionOutput = new(strings.Builder)
	err = javaCtx.Run(ctx, &biome.Invocation{
		Argv:   []string{"javac", "--version"},
		Stdout: versionOutput,
		Stderr: versionOutput,
	})
	t.Logf("javac --version:\n%s", versionOutput)
	if err != nil {
		t.Errorf("javac --version: %v", err)
	}
	if got := versionOutput.String(); !strings.Contains(got, majorVersion) {
		t.Errorf("javac --version output does not include %q", majorVersion)
	}
}

func TestJavaDownloadURL(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{
			version: "9+181",
			want:    "https://github.com/AdoptOpenJDK/openjdk9-binaries/releases/download/jdk-9%2B181/OpenJDK9U-jdk_x64_linux_hotspot_9_181.tar.gz",
		},
		{
			version: "8.252+09",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u252-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u252b09.tar.gz",
		},
		{
			version: "8.252.9",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u252-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u252b09.tar.gz",
		},
		{
			version: "8.252.09",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u252-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u252b09.tar.gz",
		},
		{
			version: "8.242+08",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "8.242.8",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "8.242.08",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "8.232+09",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u232-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u232b09.tar.gz",
		},
		{
			version: "8.232.09",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u232-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u232b09.tar.gz",
		},
		{
			version: "8.232.9",
			want:    "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u232-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u232b09.tar.gz",
		},
		{
			version: "11.0.6",
			want:    "https://github.com/AdoptOpenJDK/openjdk11-binaries/releases/download/jdk-11.0.6%2B10/OpenJDK11U-jdk_x64_linux_hotspot_11.0.6_10.tar.gz",
		},
		{
			version: "11.0.6+10",
			want:    "https://github.com/AdoptOpenJDK/openjdk11-binaries/releases/download/jdk-11.0.6%2B10/OpenJDK11U-jdk_x64_linux_hotspot_11.0.6_10.tar.gz",
		},
		{
			version: "12.0.2+10",
			want:    "https://github.com/AdoptOpenJDK/openjdk12-binaries/releases/download/jdk-12.0.2%2B10/OpenJDK12U-jdk_x64_linux_hotspot_12.0.2_10.tar.gz",
		},
		{
			version: "13.0.2+8",
			want:    "https://github.com/AdoptOpenJDK/openjdk13-binaries/releases/download/jdk-13.0.2%2B8/OpenJDK13U-jdk_x64_linux_hotspot_13.0.2_8.tar.gz",
		},
		{
			version: "13.0.1+9",
			want:    "https://github.com/AdoptOpenJDK/openjdk13-binaries/releases/download/jdk-13.0.1%2B9/OpenJDK13U-jdk_x64_linux_hotspot_13.0.1_9.tar.gz",
		},
		{
			version: "14",
			want:    "https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_x64_linux_hotspot_14_36.tar.gz",
		},
		{
			version: "14+36",
			want:    "https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_x64_linux_hotspot_14_36.tar.gz",
		},
	}
	for _, test := range tests {
		desc := &biome.Descriptor{
			OS:   biome.Linux,
			Arch: biome.Intel64,
		}
		got, err := javaDownloadURL(test.version, desc)
		if got != test.want || err != nil {
			t.Errorf("javaDownloadURL(%q, %+v) =\n%q, %v; want\n%q, <nil>", test.version, desc, got, err, test.want)
		}
	}

	t.Run("Existence", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping due to -short")
		}
		for _, test := range tests {
			verifyURLExists(t, http.MethodGet, test.want)
		}
	})
}
