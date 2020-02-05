package runtime

import (
	"fmt"
	"github.com/google/shlex"
	"github.com/yourbase/yb/plumbing/log"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
)

var (
	runtimeEnvironment = NewRuntime("uuid", "/tmp")
)


func SetEnv(key string, value string) error {
	runtimeEnvironment.DefaultTarget.SetEnv(key, value)
	return nil
}

func ExecToStdoutWithExtraEnv(cmdString string, targetDir string, env []string) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined")
	}

	return ExecToStdoutWithEnv(cmdString, targetDir, env)
}

func ExecToStdoutWithEnv(cmdString string, targetDir string, env []string) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined")
	}

	log.Infof("Running: %s in %s", cmdString, targetDir)

	p := Process{
		Command:     cmdString,
		Directory:   targetDir,
		Interactive: true,
		Environment: env,
	}

	if err := runtimeEnvironment.Run(p); err != nil {
		return fmt.Errorf("Command failed to run with error: %v", err)
	}

	return nil
}

func ExecToStdout(cmdString string, targetDir string) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined")
	}

	return ExecToStdoutWithEnv(cmdString, targetDir, os.Environ())
}

func ExecToLog(cmdString string, targetDir string, logPath string) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined")
	}

	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	logfile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open log file %s: %v", logPath, err)
	}

	defer logfile.Close()

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	cmd.Stdout = logfile
	cmd.Stdin = os.Stdin
	cmd.Stderr = logfile

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command '%s' failed to run with error -- see log for information: %s", cmdString, logPath)
	}

	return nil

}

func ExecSilently(cmdString string, targetDir string) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined")
	}

	return ExecSilentlyToWriter(cmdString, targetDir, ioutil.Discard)
}

func ExecSilentlyToWriter(cmdString string, targetDir string, writer io.Writer) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined")
	}

	cmdArgs, err := shlex.Split(cmdString)
	if err != nil {
		return fmt.Errorf("Can't parse command string '%s': %v", cmdString, err)
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
	cmd.Dir = targetDir
	cmd.Stdout = writer
	cmd.Stdin = os.Stdin
	cmd.Stderr = writer

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command '%s' failed to run with error %v", cmdString, err)
	}

	return nil

}

func ExecToLogWithProgressDots(cmdString string, targetDir string, logPath string) error {
	if runtimeEnvironment == nil {
		return fmt.Errorf("No runtime environment determined!")
	}

	stoppedchan := make(chan struct{})
	dotchan := make(chan int)
	defer close(stoppedchan)

	go func() {
		for {
			select {
			default:
				dotchan <- 1
				time.Sleep(3 * time.Second)
			case <-stoppedchan:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			default:
			case <-dotchan:
				fmt.Printf(".")
			case <-stoppedchan:
				fmt.Printf(" done!\n")
				return
			}
		}
	}()

	return ExecToLog(cmdString, targetDir, logPath)
}

func Init(identifier string, localWorkDir string) {
	runtimeEnvironment = NewRuntime(identifier, localWorkDir)
}
