package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/matishsiao/goInfo"

	"github.com/johnewart/subcommands"
	ybconfig "github.com/yourbase/yb/config"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/workspace"
)

const TIME_FORMAT = "15:04:05 MST"

type BuildCmd struct {
	Channel          string
	Version          string
	ExecPrefix       string
	NoContainer      bool
	DependenciesOnly bool
	CleanBuild       bool
}

type BuildLog struct {
	Contents string `json:"contents"`
	UUID     string `json:"uuid"`
}

func (*BuildCmd) Name() string     { return "build" }
func (*BuildCmd) Synopsis() string { return "Build the workspace" }
func (*BuildCmd) Usage() string {
	return `build`
}

func (b *BuildCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&b.NoContainer, "no-container", false, "Bypass container even if specified")
	f.BoolVar(&b.DependenciesOnly, "deps-only", false, "Install only dependencies, don't do anything else")
	f.StringVar(&b.ExecPrefix, "exec-prefix", "", "Add a prefix to all executed commands (useful for timing or wrapping things)")
	f.BoolVar(&b.CleanBuild, "clean", false, "Perform a completely clean build -- don't reuse anything when building")
}

func (b *BuildCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	startTime := time.Now()

	if InsideTheMatrix() {
		log.StartSection("BUILD CONTAINER", "CONTAINER")
	} else {
		log.StartSection("BUILD HOST", "HOST")
	}
	gi := goInfo.GetInfo()
	gi.VarDump()
	log.EndSection()

	log.StartSection("BUILD PACKAGE SETUP", "SETUP")
	log.Infof("Build started at %s", startTime.Format(TIME_FORMAT))

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}
	// Determine build target
	buildTargetName := "default"

	if len(f.Args()) > 0 {
		buildTargetName = f.Args()[0]
	}

	var targetTimers []workspace.TargetTimer

	buildFlags := workspace.BuildFlags{
		HostOnly:   b.NoContainer,
		CleanBuild: b.CleanBuild,
	}
	stepTimers, buildError := targetPackage.BuildTarget(buildTargetName, buildFlags)

	if err != nil {
		log.Errorf("Failed to build target package: %v\n", err)
		return subcommands.ExitFailure
	}

	targetTimers = append(targetTimers, workspace.TargetTimer{Name: buildTargetName, Timers: stepTimers})

	endTime := time.Now()
	buildTime := endTime.Sub(startTime)
	time.Sleep(100 * time.Millisecond)

	log.Info("")
	log.Infof("Build finished at %s, taking %s", endTime.Format(TIME_FORMAT), buildTime)
	log.Info("")
	log.Infof("%15s%15s%15s%24s   %s", "Start", "End", "Elapsed", "Target", "Command")
	for _, timer := range targetTimers {
		for _, step := range timer.Timers {
			elapsed := step.EndTime.Sub(step.StartTime).Truncate(time.Microsecond)
			log.Infof("%15s%15s%15s%24s   %s",
				step.StartTime.Format(TIME_FORMAT),
				step.EndTime.Format(TIME_FORMAT),
				elapsed,
				timer.Name,
				step.Command)
		}
	}
	log.Infof("%15s%15s%15s   %s", "", "", buildTime.Truncate(time.Millisecond), "TOTAL")

	if buildError != nil {
		log.SubSection("BUILD FAILED")
		log.Errorf("Build terminated with the following error: %v", buildError)
	} else {
		log.SubSection("BUILD SUCCEEDED")
	}

	// Output duplication start
	realStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputs := io.MultiWriter(realStdout)
	var buf bytes.Buffer
	uploadBuildLogs := ybconfig.ShouldUploadBuildLogs()

	if uploadBuildLogs {
		outputs = io.MultiWriter(realStdout, &buf)
	}

	c := make(chan bool)

	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		for {
			select {
			case <-c:
				return
			case <-time.After(100 * time.Millisecond):
				io.Copy(outputs, r)
			}
		}
	}()
	defer func() {
		w.Close()
		io.Copy(outputs, r)
		close(c)
		r.Close()
	}()
	// Output duplication end

	time.Sleep(10 * time.Millisecond)
	// Reset stdout
	//os.Stdout = realStdout

	if uploadBuildLogs {
		UploadBuildLogsToAPI(&buf)
	}

	if buildError != nil {
		return subcommands.ExitFailure
	}

	// No errors, :+1:
	return subcommands.ExitSuccess
}

func UploadBuildLogsToAPI(buf *bytes.Buffer) {
	log.Infof("Uploading build logs...")
	buildLog := BuildLog{
		Contents: buf.String(),
	}
	jsonData, _ := json.Marshal(buildLog)
	resp, err := postJsonToApi("/buildlogs", jsonData)

	if err != nil {
		log.Errorf("Couldn't upload logs: %v", err)
		return
	}

	if resp.StatusCode != 200 {
		log.Warnf("Status code uploading log: %d", resp.StatusCode)
		return
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("Couldn't read response body: %s", err)
			return
		}

		var buildLog BuildLog
		err = json.Unmarshal(body, &buildLog)
		if err != nil {
			log.Errorf("Failed to parse response: %v", err)
			return
		}

		logViewPath := fmt.Sprintf("/buildlogs/%s", buildLog.UUID)
		buildLogUrl, err := ybconfig.ManagementUrl(logViewPath)

		if err != nil {
			log.Errorf("Unable to determine build log url: %v", err)
		}

		log.Infof("View your build log here: %s", buildLogUrl)
	}

}
