package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func ExecToStdout(cmdString string, targetDir string) error {
	fmt.Printf("Running: %s\n", cmdString)

	cmdArgs := strings.Fields(cmdString)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	stdoutIn, _ := cmd.StdoutPipe()

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
		}
	}()

	if err = cmd.Wait(); err != nil {
		fmt.Printf("Command failed with %s\n", err)
		return err
	}

	//outStr := string(stdoutBuf.Bytes())
	//fmt.Printf("\nout:\n%s\n", outStr)

	return nil

}
