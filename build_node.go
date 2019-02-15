package main

import (
	"bytes"
	"fmt"
	"github.com/mholt/archiver"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var DIST_MIRROR = "https://nodejs.org/dist"

type NodeBuildTool struct {
	BuildTool
	_version      string
	_instructions BuildInstructions
}

func NewNodeBuildTool(instructions BuildInstructions) NodeBuildTool {
	parts := strings.Split(instructions.Build.Tool, ":")
	version := parts[1]

	tool := NodeBuildTool{
		_version:      version,
		_instructions: instructions,
	}

	return tool
}

func (bt NodeBuildTool) Instructions() BuildInstructions {
	return bt._instructions
}

func (bt NodeBuildTool) Version() string {
	return bt._version
}

func (bt NodeBuildTool) DoBuild() (bool, error) {
	instructions := bt.Instructions()
	version := bt.Version()

	workspace := LoadWorkspace()
	buildDir := fmt.Sprintf("%s/build", workspace.Path)
	arch := "x64"
	osName := "linux"
	nodePkgVersion := fmt.Sprintf("node-v%s-%s-%s", version, osName, arch)
	cmdPath := fmt.Sprintf("%s/%s", buildDir, nodePkgVersion)

	if _, err := os.Stat(cmdPath); err == nil {
		fmt.Printf("Node v%s located in %s!\n", version, cmdPath)
	} else {
		fmt.Printf("Would install Node v%s into %s\n", version, buildDir)
		fmt.Printf("Instructions: %s\n", instructions)
		archiveFile := fmt.Sprintf("%s.tar.gz", nodePkgVersion)
		downloadUrl := fmt.Sprintf("%s/v%s/%s", DIST_MIRROR, version, archiveFile)
		localFile := filepath.Join(buildDir, archiveFile)
		fmt.Printf("Downloading from URL %s to local file %s\n", downloadUrl, localFile)
		err := DownloadFile(localFile, downloadUrl)
		if err != nil {
			fmt.Printf("Unable to download: %v\n", err)
			return false, err
		}

		err = archiver.Unarchive(localFile, buildDir)
		if err != nil {
			fmt.Printf("Unable to decompress: %v\n", err)
			return false, err
		}
	}

	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", cmdPath, currentPath)
	fmt.Printf("Setting PATH to %s\n", newPath)
	os.Setenv("PATH", newPath)

	targetDir := filepath.Join(workspace.Path, workspace.Target)
	fmt.Printf("Working in %s...\n", targetDir)

	for _, cmdName := range instructions.Build.Commands {
		fmt.Printf("Running: %s\n", cmdName)

		cmdArgs := strings.Fields(cmdName)
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:len(cmdArgs)]...)
		cmd.Dir = targetDir
		stdoutIn, _ := cmd.StdoutPipe()

		var stdoutBuf bytes.Buffer

		stdout := io.MultiWriter(os.Stdout, &stdoutBuf)

		err := cmd.Start()

		if err != nil {
			log.Fatalf("cmd.Start() failed with '%s'\n", err)
		}

		_, err = io.Copy(stdout, stdoutIn)
		outStr := string(stdoutBuf.Bytes())
		fmt.Printf("\nout:\n%s\n", outStr)
	}
	return true, nil
}
