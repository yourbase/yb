// +build darwin

package buildpacks

func OS() string {
	return "darwin"
}

func Arch() string {
	return "amd64"
}
