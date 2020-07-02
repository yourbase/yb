package buildpacks

import (
	"context"
	"testing"
)

func TestOpenJDKUrlGeneration(t *testing.T) {
	for _, data := range []struct {
		version string
		url     string
	}{
		{
			version: "8.242.08",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "11.0.6",
			url:     "https://github.com/AdoptOpenJDK/openjdk11-binaries/releases/download/jdk-11.0.6%2B10/OpenJDK11U-jdk_x64_linux_hotspot_11.0.6_10.tar.gz",
		},
		{
			version: "11.0.6+10",
			url:     "https://github.com/AdoptOpenJDK/openjdk11-binaries/releases/download/jdk-11.0.6%2B10/OpenJDK11U-jdk_x64_linux_hotspot_11.0.6_10.tar.gz",
		},
		{
			version: "14",
			url:     "https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_x64_linux_hotspot_14_36.tar.gz",
		},
		{
			version: "14+36",
			url:     "https://github.com/AdoptOpenJDK/openjdk14-binaries/releases/download/jdk-14%2B36/OpenJDK14U-jdk_x64_linux_hotspot_14_36.tar.gz",
		},
	} {
		bt := NewJavaBuildTool(BuildToolSpec{Tool: "java", Version: data.version, PackageDir: "/opt/tools/java"})

		url, err := bt.DownloadURL(context.Background())
		if err != nil {
			t.Fatalf("Template wasn't applied correctly: %v", err)
		}
		wanted := data.url

		if url != wanted {
			t.Errorf("Wanted: '%s'; got '%s'", wanted, url)
		}
	}
}
