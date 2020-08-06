package cli

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"github.com/yourbase/yb/plumbing/log"
)

func fastFindAncestor(r *git.Repository) (h plumbing.Hash, c int, branchName string, err error) {
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
		log.Errorf("%v", err)
		return
	}

	remoteBranch, err := findRemoteBranch(ref, r)
	if err != nil {
		log.Errorf("Unable to find remote branch %v: %v", ref, err)
		return
	}
	branchName = ref.Name().Short()
	if remoteBranch == nil {
		if branchName == "master" {
			// Well, we don't need to try it again
			log.Errorln("Unable to find remote master branch")
			return
		}
		//Search again, on master
		ref, err = r.Reference(plumbing.NewBranchReferenceName("master"), false)
		if err != nil {
			log.Errorf("%v", err)
			return
		}
		remoteBranch, err = findRemoteBranch(ref, r)
		if err != nil {
			log.Errorf("Unable to find remote branch %v: %v", ref, err)
			return
		}
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
		log.Infof("Merge-base commit #%d '%v': %v", i, ancestor.Hash.String(), strings.ReplaceAll(fmt.Sprintf("%12s (...)", ancestor.Message), "\n", " "))
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
			log.Infof("Remote found for branch '%v': %v", branchName, remoteInfo)
		} else {
			log.Errorf("Unable to parse remote branch '%v'", branchName)
		}
	}

	// Count commits between head and 'h'
	commitIter, err := r.Log(&git.LogOptions{})
	if err != nil {
		return
	} else {
		x := 0
		_ = commitIter.ForEach(func(cmt *object.Commit) error {
			x++
			if cmt.Hash.String() == h.String() {
				// Stop here
				c = x
				return storer.ErrStop
			}
			return nil
		})
	}

	return
}

func findRemoteBranch(reference *plumbing.Reference, r *git.Repository) (remoteBranch *plumbing.Reference, err error) {
	branchIter, err := r.References()
	if err != nil {
		return
	}
	// If branch is main, just point to it directly
	remoteDefault := "refs/remote/origin/main"
	remoteBranch, err = r.Reference(plumbing.ReferenceName(remoteDefault), false)
	if err == nil {
		return
	}
	// If branch is master, just point to it directly
	remoteDefault = "refs/remote/origin/master"
	remoteBranch, err = r.Reference(plumbing.ReferenceName(remoteDefault), false)
	if err == nil {
		return
	}
	// We have some issues here
	// TODO(ch2036): Maybe switch to using `git` utility directly
	err = branchIter.ForEach(func(rem *plumbing.Reference) error {
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
