package buildpacks

import "testing"

func TestOpenJDKUrlGeneration(t *testing.T) {
	// NOTE This can go on forever, so we better come up with a generic and reliable way of keep tools versions up to date
	for _, data := range []struct {
		version string
		url     string
	}{
		{
			version: "9+181",
			url:     "https://github.com/AdoptOpenJDK/openjdk9-binaries/releases/download/jdk-9%2B181/OpenJDK9U-jdk_x64_linux_hotspot_9_181.tar.gz",
		},
		{
			version: "8.252+09",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u252-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u252b09.tar.gz",
		},
		{
			version: "8.252.9",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u252-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u252b09.tar.gz",
		},
		{
			version: "8.252.09",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u252-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u252b09.tar.gz",
		},
		{
			version: "8.242+08",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "8.242.8",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "8.242.08",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u242-b08/OpenJDK8U-jdk_x64_linux_hotspot_8u242b08.tar.gz",
		},
		{
			version: "8.232+09",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u232-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u232b09.tar.gz",
		},
		{
			version: "8.232.09",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u232-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u232b09.tar.gz",
		},
		{
			version: "8.232.9",
			url:     "https://github.com/AdoptOpenJDK/openjdk8-binaries/releases/download/jdk8u232-b09/OpenJDK8U-jdk_x64_linux_hotspot_8u232b09.tar.gz",
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
			version: "12.0.2+10",
			url:     "https://github.com/AdoptOpenJDK/openjdk12-binaries/releases/download/jdk-12.0.2%2B10/OpenJDK12U-jdk_x64_linux_hotspot_12.0.2_10.tar.gz",
		},
		{
			version: "13.0.2+8",
			url:     "https://github.com/AdoptOpenJDK/openjdk13-binaries/releases/download/jdk-13.0.2%2B8/OpenJDK13U-jdk_x64_linux_hotspot_13.0.2_8.tar.gz",
		},
		{
			version: "13.0.1+9",
			url:     "https://github.com/AdoptOpenJDK/openjdk13-binaries/releases/download/jdk-13.0.1%2B9/OpenJDK13U-jdk_x64_linux_hotspot_13.0.1_9.tar.gz",
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
		bt := newJavaBuildTool(buildToolSpec{tool: "java", version: data.version, sharedCacheDir: "/tmp/ybcache", packageCacheDir: "/tmp/pkgcache", packageDir: "/opt/tools/java"})

		url := bt.downloadURL()
		wanted := data.url

		if url != wanted {
			t.Errorf("Wanted:\n'%s'; got:\n'%s'", wanted, url)
		}
	}
}
