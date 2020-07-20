package runtime

import (
	"fmt"
	"strings"

	"github.com/google/shlex"
)

type distro int

type command struct {
	parts    []string
	cowPower bool
	valid    bool
}

// newCommand returns a command struct after checking each command string for package manager "names"
// it also checks for valid style, using shlex
func newCommand(cmd string) (command, error) {
	// A little, at first, naive but realistic table
	// of Linux package managers
	privileged := []string{
		"apt",      // Debian based
		"dpkg",     // Debian based
		"apk",      // Alpine
		"yum",      // Redhat/CentOS based
		"rpm",      // Redhat/CentOS based
		"urpmi",    // Mageia
		"zipper",   // openSUSE
		"dnf",      // Fedora
		"pacman",   // Arch
		"xbps",     // Void
		"brew",     // Homebrew
		"nix-env",  // Nix from NixOS
		"emerge",   // Gentoo
		"ebuild",   // Gentoo
		"cave",     // Exherbo
		"slackpkg", // Slackware
	}
	words, err := shlex.Split(cmd)
	c := command{parts: words, valid: true}

	if err != nil {
		c.valid = false
		return c, err
	}

	for _, word := range words {
		for _, pkg := range privileged {
			if priv := strings.HasPrefix(word, pkg); priv {
				c.cowPower = priv
				return c, nil
			}
		}
	}

	return c, nil
}

func privilegedCommands(cmdString string) ([]string, error) {
	cmd, err := newCommand(cmdString)
	if err != nil {
		return nil, fmt.Errorf("parsing shell like syntax of: %s: %w", cmdString, err)
	}
	var cmdArgs []string
	if cmd.valid && cmd.cowPower {
		// Should use sudo or su
		if cmd.parts[0] != "sudo" {
			cmdArgs = append(cmdArgs, "sudo")
		}
	}
	cmdArgs = append(cmdArgs, cmd.parts...)
	return cmdArgs, nil
}
