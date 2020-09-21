// +build linux

package buildpacks

import (
	"bufio"
	"os"
	"strings"
)

func OS() string {
	return "linux"
}

func Arch() string {
	return "amd64"
}

func OSVersion() string {
	// Ubuntu
	return getDebianOrUbuntuVersion()
}

func getDebianOrUbuntuVersion() string {
	f, err := os.OpenFile("/etc/os-release", os.O_RDONLY, os.ModePerm)
	if err != nil {
		return ""
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return ""
		}
		if strings.HasPrefix(line, "VERSION_CODENAME") {
			line = strings.TrimSuffix(line, "\n")
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}
}
