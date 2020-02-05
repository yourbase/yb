package runtime

// Currently just a passthrough
func ExecInSandbox(command string, workingDir string) error {
	return runtime.ExecToStdout(command, workingDir)
}
