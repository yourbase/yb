package cli

import (
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"github.com/yourbase/yb/plumbing/log"
)

/*
type BestAncestorCmd struct {
	target string
}

func (*BestAncestorCmd) Name() string { return "best-ancestor" }
func (*BestAncestorCmd) Synopsis() string {
	return "Finds de best common ancestor between HEAD and default remote's commit list"
}
func (p *BestAncestorCmd) SetFlags(f *flag.FlagSet) {}
func (*BestAncestorCmd) Usage() string {
	return "Build remotely using YB infrastructure"
}

func (b *BestAncestorCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf("%v", err)
		return subcommands.ExitFailure
	}

	repoDir := targetPackage.Path
	repo, err := git.PlainOpen(repoDir)
	commonHash := fastFindAncestor(repo)

	log.Infof("Common ancestor is '%s'", commonHash.String())

	return subcommands.ExitSuccess
}*/

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
		log.Infof("Ancestor #%d: %v", i, ancestor)
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
