// +build linux

package buildpacks

func OS() string {
	return "linux"
}

func Arch() string {
	return "amd64"
}
