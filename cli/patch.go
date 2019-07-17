package cli

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

type PatchCmd struct {
	targetRepository string
	patchFile        string
	all              bool
}

func (*PatchCmd) Name() string     { return "patch" }
func (*PatchCmd) Synopsis() string { return "patch args to the defined patch file." }
func (*PatchCmd) Usage() string {
	return `patch -target <target repository path> -out <patch file> `
}

func (p *PatchCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.targetRepository, "target", "", "Repository to generate a patch for")
	f.StringVar(&p.patchFile, "out", "", "Output file for the patch")
	f.BoolVar(&p.all, "all", false, "Consider unstaged files too")
}

func (p *PatchCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	if len(p.targetRepository) == 0 || len(p.patchFile) == 0 {
		fmt.Println(p.Usage())
		return subcommands.ExitFailure
	}

	var targetPackage Package

	// check if we're just a package
	if PathExists(MANIFEST_FILE) {
		currentPath, _ := filepath.Abs(".")
		_, pkgName := filepath.Split(currentPath)
		pkg, err := LoadPackage(pkgName, currentPath)
		if err != nil {
			fmt.Printf("Error loading package '%s': %v\n", pkgName, err)
			return subcommands.ExitFailure
		}
		targetPackage = pkg
	} else {

		workspace, err := LoadWorkspace()

		if err != nil {
			fmt.Printf("No package here, and no workspace, nothing to build!")
			return subcommands.ExitFailure
		}

		pkg, err := workspace.TargetPackage()
		if err != nil {
			fmt.Printf("Can't load workspace's target package: %v\n", err)
			return subcommands.ExitFailure
		}

		targetPackage = pkg
	}

	targetDir := p.targetRepository
	patchFile := p.patchFile

	fmt.Printf("Generating diff from local target package %s against %s and writing out to %s...\n", targetPackage.Path, targetDir, patchFile)

	repoDir, err := filepath.Abs(targetPackage.Path)
	if err != nil {
		fmt.Printf("Can't get repo dir: %v!\n", err)
		return subcommands.ExitFailure
	}

	workRepo, err := git.PlainOpen(repoDir)

	if err != nil {
		fmt.Printf("Error opening repository %s: %v\n", repoDir, err)
		return subcommands.ExitFailure
	}

	targetRepoDir, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Printf("Can't get target repo dir: %v!\n", err)
		return subcommands.ExitFailure
	}

	if repoDir == targetRepoDir {
		fmt.Printf("Cowardly decided not to generate this patch, as target is equal to package: %v == %v\n", repoDir, targetRepoDir)
		return subcommands.ExitFailure
	}

	targetRepo, err := git.PlainOpen(targetRepoDir)

	if err != nil {
		fmt.Printf("Error opening repository %s: %v\n", targetRepoDir, err)
		return subcommands.ExitFailure
	}

	// Get set of commits in the target (where we're going to patch on top of)
	targetSet := commitSet(targetRepo)
	if targetSet == nil {
		fmt.Printf("Unable to build a commit set for comparing\n")
		return subcommands.ExitFailure
	}

	commonAncestor := findCommonAncestor(workRepo, targetSet)
	headRef, _ := workRepo.Head()

	baseCommit, _ := workRepo.CommitObject(commonAncestor)
	headCommit, _ := workRepo.CommitObject(headRef.Hash())

	LOGGER.Debugf("Comparing base commit '%v' to head commit (local directory) '%v'", baseCommit.String(), headCommit.String())

	stagedLocal := false
	// TODO Stage a dirty worktree base dir
	worktree, err := workRepo.Worktree()
	if err == nil {
		if p.all {
			// To get untracked, unstaged, staged and other changes between, we have to use the plumbing.Tree object

			// baseTree := workRepo.TreeObjects()

			if status, err := worktree.Status(); err != nil {
				fmt.Printf("Status unavailable, check your .git dir: %v\n", err)
			} else {
				if !status.IsClean() {
					// Stages changes for them to be included in the patch
					// Later we should unstage them
					if _, err := worktree.Add("."); err != nil {
						fmt.Printf("Couldn't temporarily stage changes: %v\n", err)
					} else {
						stagedLocal = true
					}
				}
			}
		}
	} else {
		fmt.Printf("Unable to verify worktree's state, unstaged changes ignored: %v\n", err)
	}

	commitPatch, _ := baseCommit.Patch(headCommit)

	fmt.Printf("Writing patch to %s\n", patchFile)
	ioutil.WriteFile(patchFile, []byte(commitPatch.String()), 0600)

	if stagedLocal {
		// Unstage changes
		if err = worktree.Reset(&git.ResetOptions{Mode: git.MixedReset}); err != nil {
			fmt.Printf("Unable to unstage local changes, sorry for the unconvenience")
		}
	}

	return subcommands.ExitSuccess
}

func findCommonAncestor(r *git.Repository, commits map[string]bool) plumbing.Hash {
	ref, err := r.Head()
	if err != nil {
		fmt.Errorf("No Head: %v\n", err)
	}

	commit, _ := r.CommitObject(ref.Hash())
	commitIter, _ := r.Log(&git.LogOptions{From: commit.Hash})
	var commonCommit *object.Commit

	err = commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		if LOGGER.LogDebug() {
			LOGGER.Debugf("Considering %s -> %v...\n", hash, commits[hash])
		}
		if commits[hash] {
			commonCommit = c
			return storer.ErrStop
		} else {
			return nil
		}
	})

	if err != nil {
		LOGGER.Debugf("Error printing: %v\n", err)
	}

	LOGGER.Debugf("Common commit hash: %s\n", commonCommit.Hash)
	return commonCommit.Hash

}

func commitSet(r *git.Repository) map[string]bool {
	if r == nil {
		fmt.Printf("Error getting the repo\n")
		return nil
	}
	ref, err := r.Head()

	if err != nil {
		fmt.Printf("No Head: %v\n", err)
		return nil
	}

	commit, _ := r.CommitObject(ref.Hash())
	commitIter, _ := r.Log(&git.LogOptions{From: commit.Hash})
	hashSet := make(map[string]bool)

	err = commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		hashSet[hash] = true
		return nil
	})

	if err != nil {
		fmt.Printf("Error printing: %v\n", err)
	}

	return hashSet

}
