package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"github.com/yourbase/yb/plumbing/log"
)

func (p *RemoteCmd) fastFindAncestor(ctx context.Context, r *git.Repository) (h plumbing.Hash, c int, branchName string, err error) {
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
	branchName = ref.Name().Short()

	remoteBranch, err := p.findRemoteBranch(ctx, ref, r)
	if err != nil {
		log.Errorf("Unable to find remote branch for %v: %v", branchName, err)
		return
	}
	if remoteBranch == nil {
		log.Errorf("Unable to find remote branch for %v", branchName)
		return
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
	}
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

	return
}

func (p *RemoteCmd) findRemoteBranch(ctx context.Context, reference *plumbing.Reference, r *git.Repository) (remoteBranch *plumbing.Reference, err error) {
	remotes, err := r.Remotes()
	if err != nil || len(remotes) == 0 {
		return
	}
	remoteName := remotes[0].Config().Name
	refName := reference.Name().Short()
	remoteDefault := "refs/remotes/" + remoteName + "/" + refName
	remoteBranch, err = r.Reference(plumbing.ReferenceName(remoteDefault), false)
	if err == nil {
		return
	}

	remoteHEAD := plumbing.NewRemoteHEADReferenceName(remoteName)
	remoteHEADReference, err := r.Reference(remoteHEAD, true)
	if err == nil {
		remoteDefault = "refs/remotes/" + remoteHEADReference.Name().Short() // Should be "origin/main", but can be other names defined by the user as well
		remoteBranch, err = r.Reference(plumbing.ReferenceName(remoteDefault), false)
		if err == nil {
			return
		}
	}
	err = nil

	if !p.goGit {
		const (
			gitBranchCmd = " branch --remote"
		)

		buf := bytes.NewBuffer(nil)
		cmdString := gitExe + gitBranchCmd
		log.Debugf("Executing '%v'...", cmdString)
		if err = p.metalTgt.ExecSilentlyToWriter(ctx, cmdString, p.repoDir, buf); err != nil {
			err = fmt.Errorf("git branch: %w", err)
			return
		}

		if buf.Len() == 0 {
			err = fmt.Errorf("git branch: empty response")
			return
		}
		for {
			line, rerr := buf.ReadString('\n')
			if rerr != nil {
				if errors.Is(rerr, io.EOF) {
					err = fmt.Errorf("reached %w", rerr)
				} else {
					err = rerr
				}
				return
			}
			line = strings.Trim(line, " \n")
			bName := strings.ReplaceAll(line, remoteName+"/", "")
			log.Debugf("Branch found: %s ?== %s", bName, reference.Name().Short())
			if strings.HasSuffix(bName, reference.Name().Short()) {
				remoteBranch, rerr = r.Reference(plumbing.ReferenceName(bName), true)
				if rerr != nil {
					err = rerr
					return
				}
				return
			}
		}
	}
	return
}
