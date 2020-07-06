package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	// nit: It's preferred to use the import "golang.org/x/sys/unix" to indicate that this package requires Unix-specific symbols.
	"syscall"
	"time"

	"github.com/johnewart/subcommands"

	ybconfig "github.com/yourbase/yb/config"
	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/remote"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
	"github.com/yourbase/yb/workspace"
)

type RemoteCmd struct {
	target         string
	baseCommit     string
	branch         string
	patchData      []byte
	patchPath      string
	repoDir        string
	printStatus    bool
	goGitStatus    bool
	noAcceleration bool
	disableCache   bool
	disableSkipper bool
	dryRun         bool
	committed      bool
	publicRepo     bool
	backupWorktree bool
}

func (*RemoteCmd) Name() string     { return "remotebuild" }
func (*RemoteCmd) Synopsis() string { return "Build remotely." }
func (*RemoteCmd) Usage() string {
	return "Build remotely using YB infrastructure"
}

func (p *RemoteCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.baseCommit, "base-commit", "", "Base commit hash as common ancestor")
	f.StringVar(&p.branch, "branch", "", "Branch name")
	f.StringVar(&p.patchPath, "patch-path", "", "Path to save the patch")
	f.BoolVar(&p.noAcceleration, "no-accel", false, "Disable acceleration")
	f.BoolVar(&p.disableCache, "disable-cache", false, "Disable cache acceleration")
	f.BoolVar(&p.disableSkipper, "disable-skipper", false, "Disable skipping steps acceleration")
	f.BoolVar(&p.dryRun, "dry-run", false, "Pretend to remote build")
	f.BoolVar(&p.committed, "committed", false, "Only remote build committed changes")
	f.BoolVar(&p.printStatus, "print-status", false, "Print result of `git status` used to grab untracked/unstaged changes")
	f.BoolVar(&p.goGitStatus, "go-git-status", false, "Use internal go-git.v4 status instead of calling `git status`, can be slow")
	f.BoolVar(&p.backupWorktree, "backup-worktree", false, "Saves uncommitted work into a tarball")
}

func (p *RemoteCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	// Consistent with how the `build` cmd works
	p.target = "default"
	if len(f.Args()) > 0 {
		p.target = f.Args()[0]
	}

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	manifest := targetPackage.Manifest

	var target workspace.BuildTarget

	if len(manifest.BuildTargets) == 0 {
		target = manifest.Build
		if len(target.Commands) == 0 {
			log.Errorf("Default build command has no steps and no targets described")
			return subcommands.ExitFailure
		}
	} else {
		if _, err := manifest.BuildTarget(p.target); err != nil {
			log.Errorf("Build target %s specified but it doesn't exist!", p.target)
			log.Infof("Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
			return subcommands.ExitFailure
		}
	}

	remoteBuild := remote.NewRemoteBuild()
	if err := remote.GitPrepare(ctx, targetPackage); err != nil {
		log.Errorf("Initializing: %v", err)
		return subcommands.ExitFailure
	}

	// Show timing feedback and start tracking spent time
	startTime := time.Now()
	var bootProgress *Progress
	bootErrored := func() {
		if bootProgress != nil {
			bootProgress.Fail()
		}
	}

	if log.CheckIfTerminal() {
		bootProgress = NewProgressSpinner("Bootstrapping")
		bootProgress.Start()
	} else {
		log.Info("Bootstrapping...")
	}

	// First things first:
	// 1. Define correct branch name
	// 2. Define common ancestor commit
	// 3. Generate patch file
	//    3.1. Comparing every local commits with the one upstream
	//    3.2. Comparing every unstaged/untracked changes with the one upstream
	//    3.3. Save the patch and compress it
	// 4. Submit build!

	if bootProgress != nil {
		fmt.Println()
	}

	head, _ := workRepo.Head()
	headCommit, err := workRepo.CommitObject(head.Hash())
	if err != nil {
		bootErrored()
		log.Errorf("Couldn't find HEAD commit: %v", err)
		return subcommands.ExitFailure
	}
	ancestorCommit, err := workRepo.CommitObject(ancestorRef)
	if err != nil {
		bootErrored()
		log.Errorf("Couldn't find merge-base commit: %v", err)
		return subcommands.ExitFailure
	}

	// Show feedback: end of bootstrap
	endTime := time.Now()
	bootTime := endTime.Sub(startTime)
	if bootProgress != nil {
		bootProgress.Success()
	}
	log.Infof("Bootstrap finished at %s, taking %s", endTime.Format(TIME_FORMAT), bootTime.Truncate(time.Millisecond))

	// Process patches
	startTime = time.Now()
	var patchProgress *Progress
	patchErrored := func() {
		if patchProgress != nil {
			patchProgress.Fail()
		}
	}

	// Show feedback: end of patch generation
	endTime = time.Now()
	patchTime := endTime.Sub(startTime)
	if patchProgress != nil {
		patchProgress.Success()
	}
	log.Infof("Patch finished at %s, taking %s", endTime.Format(TIME_FORMAT), patchTime.Truncate(time.Millisecond))
	if len(p.patchPath) > 0 && len(p.patchData) > 0 {
		if err := p.savePatch(); err != nil {
			if patchProgress != nil {
				fmt.Println()
			}
			log.Warningf("Unable to save copy of generated patch: %v", err)
		}
	}

	if !p.dryRun {
		err = p.submitBuild(project, target.Tags)

		if err != nil {
			log.Errorf("Unable to submit build: %v", err)
			return subcommands.ExitFailure
		}
	} else {
		log.Infoln("Dry run ended, build not submitted")
	}

	return subcommands.ExitSuccess
}
