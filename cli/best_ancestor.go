package cli

import (
	"fmt"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"github.com/yourbase/yb/plumbing/log"
)

func fastFindAncestor(r *git.Repository) (h plumbing.Hash, branchName string) {
	/*
		go doc gopkg.in/src-d/go-git.v4/plumbing/object Commit.MergeBase

		func (c *Commit) MergeBase(other *Commit) ([]*Commit, error)
		MergeBase mimics the behavior of `git merge-base actual other`, returning
		the best common ancestor between the actual and the passed one. The best
		common ancestors can not be reached from other common ancestors.
	*/

	ref, err := r.Head()
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	remoteBranch := findRemoteBranch(ref, r)
	branchName = ref.Name().Short()
	if remoteBranch == nil {
		//Search again, on master
		ref, err = r.Reference(plumbing.NewBranchReferenceName("master"), false)
		if err != nil {
			log.Errorf("%v", err)
			return
		}
		remoteBranch = findRemoteBranch(ref, r)
		branchName = ref.Name().Short() // "master"
	}

	headCommit, err := r.CommitObject(ref.Hash())
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	remoteCommit, err := r.CommitObject(remoteBranch.Hash())
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	commonAncestors, err := remoteCommit.MergeBase(headCommit)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	for i, ancestor := range commonAncestors {
		log.Infof("Merge-base commit #%d: %v", i, strings.ReplaceAll(fmt.Sprintf("%12s (...)", ancestor.Message), "\n", ""))
	}
	// For now we'll return the first one
	if len(commonAncestors) > 0 {
		h = commonAncestors[0].Hash
	}

	return
}

func findRemoteBranch(reference *plumbing.Reference, r *git.Repository) (remoteBranch *plumbing.Reference) {
	branchIter, _ := r.References()
	_ = branchIter.ForEach(func(rem *plumbing.Reference) error {
		if rem.Name().IsRemote() {
			log.Debugf("Branch found: %v, %s ?== %s", rem, rem.Name().Short(), reference.Name().Short())
			if strings.HasSuffix(rem.Name().Short(), reference.Name().Short()) {
				remoteBranch = rem
				return storer.ErrStop
			}
		}
		return nil
	})
	return
}
