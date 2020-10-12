// +build windows

package buildpack

func OS() string {
	return "windows"
}

func Arch() string {
	return "amd64"
}

func OSVersion() string {
	return "10"
}
