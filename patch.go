package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/alexcesaro/log/stdlog"
	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var LOGGER = stdlog.GetFromFlags()

type patchCmd struct {
	targetRepository string
	patchFile        string
}

func (*patchCmd) Name() string     { return "patch" }
func (*patchCmd) Synopsis() string { return "patch args to stdout." }
func (*patchCmd) Usage() string {
	return `patch -target <target repository path> -out <patch file>`
}

func (p *patchCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.targetRepository, "target", "", "Repository to generate a patch for")
	f.StringVar(&p.patchFile, "out", "", "Output file for the patch")
}

func (p *patchCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	if len(p.targetRepository) == 0 || len(p.patchFile) == 0 {
		fmt.Println(p.Usage())
		return subcommands.ExitFailure
	}

	workspace := LoadWorkspace()
	targetPackage := workspace.Target
	targetDir := p.targetRepository
	patchFile := p.patchFile

	fmt.Printf("Generating diff from local target package %s against %s and writing out to %s...\n", targetPackage, targetDir, patchFile)

	repoDir, err := filepath.Abs(targetPackage)
	if err != nil {
		fmt.Errorf("Can't get repo dir: %v!\n", err)
	}

	workRepo, err := git.PlainOpen(repoDir)

	if err != nil {
		fmt.Printf("Error opening repository %s: %v\n", repoDir, err)
	}

	targetRepo, err := git.PlainOpen(targetDir)

	if err != nil {
		fmt.Printf("Error opening repository %s: %v\n", targetDir, err)
	}

	// Get set of commits in the target (where we're going to patch on top of)
	targetSet := commitSet(targetRepo)
	commonAncestor := findCommonAncestor(workRepo, targetSet)
	headRef, _ := workRepo.Head()

	baseCommit, _ := workRepo.CommitObject(commonAncestor)
	headCommit, _ := workRepo.CommitObject(headRef.Hash())

	commitPatch, _ := baseCommit.Patch(headCommit)

	fmt.Printf("Writing patch to %s\n", patchFile)
	ioutil.WriteFile(patchFile, []byte(commitPatch.String()), 0600)

	return subcommands.ExitSuccess
}

func findCommonAncestor(r *git.Repository, commits map[string]bool) plumbing.Hash {
	ref, _ := r.Head()

	commit, _ := r.CommitObject(ref.Hash())
	commitIter, _ := r.Log(&git.LogOptions{From: commit.Hash})
	var commonCommit *object.Commit

	err := commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		LOGGER.Debug("Considering %s -> %s...\n", hash, commits[hash])
		if commits[hash] {
			commonCommit = c
			return storer.ErrStop
		} else {
			return nil
		}
	})

	if err != nil {
		fmt.Errorf("Error printing: %v\n", err)
	}

	LOGGER.Debug("Common commit hash: %s\n", commonCommit.Hash)
	return commonCommit.Hash

}

func commitSet(r *git.Repository) map[string]bool {
	ref, _ := r.Head()

	commit, _ := r.CommitObject(ref.Hash())
	commitIter, _ := r.Log(&git.LogOptions{From: commit.Hash})
	hashSet := make(map[string]bool)

	err := commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		hashSet[hash] = true
		return nil
	})

	if err != nil {
		fmt.Printf("Error printing: %v\n", err)
	}

	return hashSet

}
