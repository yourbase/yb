// +build windows

package buildpacks

func OS() string {
	return "windows"
}

func Arch() string {
	return "amd64"
}
