package plumbing

// Currently just a passthrough
func ExecInSandbox(command string, workingDir string) error {
	return ExecToStdout(command, workingDir)
}
