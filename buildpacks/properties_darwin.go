// +build darwin

package buildpacks

func OS() string {
	return "darwin"
}

func Arch() string {
	return "amd64"
}

func OSVersion() string {
	return "18.5.0"
}
