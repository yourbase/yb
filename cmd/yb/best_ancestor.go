package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"zombiezen.com/go/log"
)

func fastFindAncestor(ctx context.Context, r *git.Repository) (h plumbing.Hash, c int, branchName string, err error) {
	/*
		go doc gopkg.in/src-d/go-git.v4/plumbing/object Commit.MergeBase

		func (c *Commit) MergeBase(other *Commit) ([]*Commit, error)
		MergeBase mimics the behavior of `git merge-base actual other`, returning
		the best common ancestor between the actual and the passed one. The best
		common ancestors can not be reached from other common ancestors.
	*/
	c = -1

	ref, err := r.Head()
	if err != nil {
		err = fmt.Errorf("find ancestor: %w", err)
		return
	}

	remoteBranch := findRemoteBranch(ctx, ref, r)
	branchName = ref.Name().Short()
	if remoteBranch == nil {
		if branchName == "master" {
			// Well, we don't need to try it again
			err = fmt.Errorf("find ancestor: unable to find remote master branch")
			return
		}
		//Search again, on master
		ref, err = r.Reference(plumbing.NewBranchReferenceName("master"), false)
		if err != nil {
			err = fmt.Errorf("find ancestor: %w", err)
			return
		}
		remoteBranch = findRemoteBranch(ctx, ref, r)
		branchName = ref.Name().Short() // "master"
	}

	headCommit, err := r.CommitObject(ref.Hash())
	if err != nil {
		err = fmt.Errorf("find ancestor: %w", err)
		return
	}
	remoteCommit, err := r.CommitObject(remoteBranch.Hash())
	if err != nil {
		err = fmt.Errorf("find ancestor: %w", err)
		return
	}
	commonAncestors, err := remoteCommit.MergeBase(headCommit)
	if err != nil {
		err = fmt.Errorf("find ancestor: %w", err)
		return
	}

	for i, ancestor := range commonAncestors {
		log.Infof(ctx, "Merge-base commit #%d '%v': %v", i, ancestor.Hash.String(), strings.ReplaceAll(fmt.Sprintf("%12s (...)", ancestor.Message), "\n", " "))
	}
	// For now we'll return the first one
	if len(commonAncestors) > 0 {
		h = commonAncestors[0].Hash
	}

	// Show remote name from branch complete name
	remoteName := remoteBranch.Name().String()
	remoteRefRegexp := regexp.MustCompile(`refs/remotes/(\w+)/(\w+)`)

	if remoteRefRegexp.MatchString(remoteName) {
		submatches := remoteRefRegexp.FindStringSubmatch(remoteName)

		if len(submatches) > 1 {
			remoteInfo := submatches[1]
			if config, err := r.Config(); err == nil {
				remoteInfo = fmt.Sprintf("\"%s [ %s ]\"", remoteInfo, config.Remotes[submatches[1]].URLs[0])
			}
			log.Infof(ctx, "Remote found for branch '%v': %v", branchName, remoteInfo)
		} else {
			log.Errorf(ctx, "Unable to parse remote branch '%v'", branchName)
		}
	}

	// Count commits between head and 'h'
	commitIter, err := r.Log(&git.LogOptions{})
	if err != nil {
		err = fmt.Errorf("find ancestor: %w", err)
		return
	}
	x := 0
	err = commitIter.ForEach(func(cmt *object.Commit) error {
		x++
		if cmt.Hash.String() == h.String() {
			// Stop here
			c = x
			return storer.ErrStop
		}
		return nil
	})

	return
}

func findRemoteBranch(ctx context.Context, reference *plumbing.Reference, r *git.Repository) (remoteBranch *plumbing.Reference) {
	branchIter, _ := r.References()
	_ = branchIter.ForEach(func(rem *plumbing.Reference) error {
		if rem.Name().IsRemote() {
			log.Debugf(ctx, "Branch found: %v, %s ?== %s", rem, rem.Name().Short(), reference.Name().Short())
			if strings.HasSuffix(rem.Name().Short(), reference.Name().Short()) {
				remoteBranch = rem
				return storer.ErrStop
			}
		}
		return nil
	})
	return
}
