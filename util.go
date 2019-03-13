package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func ExecToStdout(cmdString string, targetDir string) error {
	fmt.Printf("Running: %s\n", cmdString)

	cmdArgs := strings.Fields(cmdString)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var stdoutBuf bytes.Buffer

	err := cmd.Start()

	if err != nil {
		return fmt.Errorf("cmd.Start() failed with '%s'\n", err)
	}

	go func() {
		//outputs := io.MultiWriter(os.Stdout, websock)
		outputs := io.MultiWriter(os.Stdout, &stdoutBuf)
		for {
			io.Copy(outputs, stdoutIn)
			io.Copy(outputs, stderrIn)
			time.Sleep(300 * time.Millisecond)
		}
	}()

	if err = cmd.Wait(); err != nil {
		fmt.Printf("Command failed with %s\n", err)
		outStr := string(stdoutBuf.Bytes())
		fmt.Printf("\nout:\n%s\n", outStr)

		return err
	}

	return nil

}

func PrependToPath(dir string) {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", dir, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)
}

func MkdirAsNeeded(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Printf("Making dir: %s\n", dir)
		if err := os.Mkdir(dir, 0700); err != nil {
			fmt.Printf("Unable to create dir: %v\n", err)
			return err
		}
	}

	return nil
}