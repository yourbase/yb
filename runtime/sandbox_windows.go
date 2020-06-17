package runtime

// TODO Implement sandbox
// Currently just a passthrough
func ExecInSandbox(command string, workingDir string) error {
	return nil
	// return ExecToStdout(command, workingDir)
}
