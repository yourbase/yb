package remote

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	. "github.com/yourbase/yb/types"
	"github.com/yourbase/yb/workspace"
)

var (
	target *runtime.MetalTarget
)

const (
	gitExe       = "git" // UNIX
	gitStatusCmd = "%s status --porcelain"
)

func init() {
	execRuntime := runtime.NewRuntime(ctx, "remotebuild_checks", "./")
	target = execRuntime.DefaultTarget.(*runtime.MetalTarget)
}

func (r *RemoteBuild) GitPrepare(ctx context.Context, targetPackage *workspace.Package) error {
	path := targetPackage.Path()

	//Initization finished (TODO, add progress spinner logic)

	//Git basics
	repo, err := r.workingRepo(ctx, path)
	if err != nil {
		return err
	}

	list, err := repo.Remotes()

	if err != nil {
		return fmt.Errorf("getting remotes for %s: %v", path, err)
	}

	var repoUrls []string

	for _, r := range list {
		c := r.Config()
		for _, u := range c.URLs {
			repoUrls = append(repoUrls, u)
		}
	}

	// Project settings check-up
	project, remote, err := p.fetchProject(repoUrls)
	if err != nil {
		return fmt.Errorf("fetching project: %v", err)
	}
	if !remote.Validate() {
		return fmt.Errorf("invalid or unusable remote: %v", remote)
	}
	if project.Repository == "" {
		projectUrl, err := ybconfig.ManagementUrl(fmt.Sprintf("%s/%s", project.OrgSlug, project.Label))
		if err != nil {
			return fmt.Errorf("generating project URL: %v", err)
		}

		return fmt.Errorf("empty repository for project %s, please check your project settings at %s", project.Label, projectUrl)
	}

	// Remote branch validation and asserts
	head, remote.Branch, err = validBranch(workRepo, r.BranchName)
	if err != nil {
		return fmt.Errorf("checking branch: %v", err)
	}

	ancestorRef, commitCount, remoteBranch, err := fastFindAncestor(workRepo)
	if err != nil {
		return fmt.Errorf("best ancestor: %v", err)
	}
	r.remoteBranch = remoteBranch
	r.commitCount = commitCount
	r.baseCommit = ancestorRef.String()

	//Initization finished (TODO, add progress spinner logic)

}

func (r *RemoteBuild) Patch(ctx context.Context) error {
}

func (r *RemoteBuild) createPatch(ctx context.Context, workRepo *git.Repository, headCommit, baseCommit, ancestorCommit *object.Commit) (data []byte, err error) {
	created := make(chan bool)
	patchCtx := context.WithDeadline(ctx, time.Minute*5)
	if r.onlyCommitted && headCommit.Hash.String() != r.baseCommit {
		log.Infof("Generating patch for %d commits...", r.commitCount)

		patch, err := ancestorCommit.PatchContext(patchCtx, headCommit)
		if err != nil {
			err = fmt.Errorf("patch generation failed: %v", err)
			return
		}
		// This is where the patch is actually generated see #278
		go func(ch chan<- bool) {
			log.Debug("Starting the actual patch generation...")
			p.patchData = []byte(patch.String())
			log.Debug("Patch generation finished, only committed changes")
			ch <- true
		}(created)
	} else if !p.committed {
		// Apply changes that weren't committed yet
		worktree, err := workRepo.Worktree() // current worktree
		if err != nil {
			patchErrored()
			log.Errorf("Couldn't get current worktree: %v", err)
			return subcommands.ExitFailure
		}

		if log.CheckIfTerminal() {
			patchProgress = NewProgressSpinner("Generating patch for local changes")
			patchProgress.Start()
		} else {
			log.Info("Generating patch for local changes")
		}

		log.Debug("Start backing up the worktree-save")
		saver, err := NewWorktreeSave(targetPackage.Path(), headCommit.Hash.String(), p.backupWorktree)
		if err != nil {
			patchErrored()
			log.Errorf("%s", err)
		}

		// TODO When go-git worktree.Status() get faster use this instead:
		// There's also the problem detecting CRLF in Windows text/source files
		//if err = worktree.AddGlob("."); err != nil {
		if skippedBinaries, err := p.traverseChanges(worktree, saver); err != nil {
			log.Error(err)
			patchErrored()
			return subcommands.ExitFailure
		} else {
			if len(skippedBinaries) > 0 {
				if patchProgress != nil {
					fmt.Println()
				}
				log.Infoln("Skipped binaries:")
				for _, n := range skippedBinaries {
					fmt.Printf("   '%s'\n", n)
				}
			}
		}

		resetDone := false
		if p.backupWorktree && len(saver.Files) > 0 {
			log.Debug("Saving a tarball  with all the worktree changes made")
			// Save them before committing
			if saveFile, err := saver.Save(); err != nil {
				patchErrored()
				log.Errorf("Unable to keep worktree changes, won't commit: %v", err)
				return subcommands.ExitFailure
			} else {
				defer func(s string) {
					if !resetDone {
						log.Debug("Reset failed, restoring the tarball")
						if err := saver.Restore(s); err != nil {
							log.Errorf("Unable to restore kept files at %v: %v\n     Please consider unarchiving that package", saveFile, err)
						} else {
							_ = os.Remove(s)
						}
					} else {
						_ = os.Remove(s)
					}
				}(saveFile)
			}
		}

		log.Debug("Committing temporary changes")
		latest, err := commitTempChanges(worktree, headCommit)
		if err != nil {
			log.Errorf("Commit to temporary cloned repository failed: %v", err)
			patchErrored()
			return subcommands.ExitFailure
		}

		tempCommit, err := workRepo.CommitObject(latest)
		if err != nil {
			log.Errorf("Can't find commit '%v': %v", latest, err)
			patchErrored()
			return subcommands.ExitFailure
		}

		log.Debug("Starting the actual patch generation...")
		patch, err := ancestorCommit.Patch(tempCommit)
		if err != nil {
			log.Errorf("Patch generation failed: %v", err)
			patchErrored()
			return subcommands.ExitFailure
		}

		// This is where the patch is actually generated see #278
		p.patchData = []byte(patch.String())
		log.Debug("Actual patch generation finished")

		log.Debug("Reseting worktree to previous state...")
		// Reset back to HEAD
		if err := worktree.Reset(&git.ResetOptions{
			Commit: headCommit.Hash,
		}); err != nil {
			log.Errorf("Unable to reset temporary commit: %v\n    Please try `git reset --hard HEAD~1`", err)
		} else {
			resetDone = true
		}
		log.Debug("Worktree reset done.")

	}

}

func (r *RemoteBuild) workingRepo(ctx context.Context, path string) (*git.Repository, error) {
	workRepo, err := git.PlainOpen(path)

	if err != nil {
		return nil, fmt.Errorf("opening repository %s: %v", path, err)
	}

	if !r.goGit && !r.onlyCommitted {
		// Need to check if `git` binary exists and works
		if p.metalTgt.OS() == runtime.Windows {
			gitExe = "git.exe"
		}
		cmdString := fmt.Sprintf("%s --version", gitExe)
		if err := target.ExecSilently(ctx, cmdString, p.repoDir); err != nil {
			if strings.Contains(err.Error(), "executable file not found in $PATH") {
				log.Warnf("The flag -go-git-status wasn't specified and '%s' wasn't found in PATH", gitExe)
			} else {
				log.Warnf("The flag -go-git-status wasn't specified but calling '%s' gives: %v", cmdString, err)
			}
			log.Warn("Switching to using internal go-git status to detect local changes, it can be slower")
			r.goGit = true
		}
	}
	return workRepo, nil
}

func (p *RemoteCmd) pickRemote(url string) (remote GitRemote) {

	for _, r := range p.remotes {
		if strings.Contains(url, r.Url) || strings.Contains(r.Url, url) {
			// If it matches the Url received from the API somehow:
			return r
		}
	}
	if len(p.remotes) > 0 {
		// We only support GitHub by now
		// TODO create something more generic
		for _, rem := range p.remotes {
			if strings.Contains(rem.Domain, "github.com") {
				remote = rem
				return
			}
		}
	}

	return
}

func commitTempChanges(w *git.Worktree, c *object.Commit) (latest plumbing.Hash, err error) {
	if w == nil || c == nil {
		err = fmt.Errorf("Needs a worktree and a commit object")
		return
	}
	latest, err = w.Commit(
		"YourBase remote build",
		&git.CommitOptions{
			Author: &object.Signature{
				Name:  c.Author.Name,
				Email: c.Author.Email,
				When:  time.Now(),
			},
		},
	)
	return
}

func (p *RemoteCmd) traverseChanges(worktree *git.Worktree, saver *WorktreeSave) (skipped []string, err error) {
	if p.goGitStatus {
		log.Debug("Decided to use Go-Git")
		skipped, err = p.libTraverseChanges(worktree, saver)
	} else {
		log.Debugf("Decided to use `%s`", gitStatusCmd)
		skipped, err = p.commandTraverseChanges(worktree, saver)
	}
	return
}

func shouldSkip(file string, worktree *git.Worktree) bool {
	filePath := path.Join(worktree.Filesystem.Root(), file)
	fi, err := os.Stat(filePath)
	if err != nil {
		return true
	}

	if fi.IsDir() {
		log.Debugf("Added a dir, checking it's contents: %s", file)
		f, err := os.Open(filePath)
		if err != nil {
			return true
		}
		dir, err := f.Readdir(0)
		if err != nil {
			return true
		}
		log.Debugf("Ls dir %s", filePath)
		for _, f := range dir {
			child := path.Join(file, f.Name())
			log.Debugf("Shoud skip child '%s'?", child)
			if shouldSkip(child, worktree) {
				continue
			} else {
				worktree.Add(child)
			}
		}
		return true
	} else {
		log.Debugf("Should skip '%s'?", filePath)
		if is, _ := IsBinary(filePath); is {
			return true
		}
	}
	return false
}

func (p *RemoteCmd) commandTraverseChanges(worktree *git.Worktree, saver *WorktreeSave) (skipped []string, err error) {
	// TODO When go-git worktree.Status() works faster, we'll disable this
	// Get worktree path
	repoDir := worktree.Filesystem.Root()
	cmdString := fmt.Sprintf(gitStatusCmd, gitExe)
	buf := bytes.NewBuffer(nil)
	log.Debugf("Executing '%v'...", cmdString)
	if err = p.metalTgt.ExecSilentlyToWriter(context.TODO(), cmdString, repoDir, buf); err != nil {
		return skipped, fmt.Errorf("When running git status: %v", err)
	}

	if buf.Len() > 0 {
		if p.printStatus {
			fmt.Println()
			fmt.Print(buf.String())
		}
		for {
			if line, err := buf.ReadString('\n'); len(line) > 4 && err == nil { // Line should have at least 4 chars
				line = strings.Trim(line, "\n")
				log.Debugf("Processing git status line:\n%s", line)
				mode := line[:2]
				file := line[3:]
				modUnstagedMap := map[byte]bool{'?': true, 'M': true, 'D': true, 'R': true} // 'R' isn't really found at mode[1], but...

				// Unstaged modifications of any kind
				if modUnstagedMap[mode[1]] {
					if shouldSkip(file, worktree) {
						skipped = append(skipped, file)
						continue
					}
					log.Debugf("Adding %s to the index", file)
					// Add each detected change
					if _, err = worktree.Add(file); err != nil {
						return skipped, fmt.Errorf("Unable to add %s: %v", file, err)
					}

					if mode[1] != 'D' {
						// No need to add deletion to the saver, right?
						log.Debugf("Saving %s to the tarball", file)
						if err = saver.Add(file); err != nil {
							return skipped, fmt.Errorf("Need to save state, but couldn't: %v", err)
						}
					}

				}
				// discard len(line) bytes
				//discard := make([]byte, len(line))
				//_, _ = buf.Read(discard)

			} else if err == io.EOF {
				break
			} else if err != nil {
				return skipped, fmt.Errorf("When running git status: %v", err)
			}
		}
	}
	return
}
func (p *RemoteCmd) libTraverseChanges(worktree *git.Worktree, saver *WorktreeSave) (skipped []string, err error) {
	// This could get real slow, check https://github.com/src-d/go-git/issues/844
	status, err := worktree.Status()
	if err != nil {
		log.Errorf("Couldn't get current worktree status: %v", err)
		return
	}

	if p.printStatus {
		fmt.Println()
		fmt.Print(status.String())
	}
	for n, s := range status {
		log.Debugf("Checking status %v", n)
		// Deleted (staged removal or not)
		if s.Worktree == git.Deleted {

			if _, err = worktree.Remove(n); err != nil {
				err = fmt.Errorf("Unable to remove %s: %v", n, err)
				return
			}
		} else if s.Staging == git.Deleted {
			// Ignore it
		} else if s.Worktree == git.Renamed || s.Staging == git.Renamed {

			log.Debugf("Saving %s to the tarball", n)
			if err = saver.Add(n); err != nil {
				err = fmt.Errorf("Need to save state, but couldn't: %v", err)
				return
			}

			if _, err = worktree.Move(s.Extra, n); err != nil {
				err = fmt.Errorf("Unable to move %s -> %s: %v", s.Extra, n, err)
				return
			}
		} else {
			if shouldSkip(n, worktree) {
				skipped = append(skipped, n)
				continue
			}
			log.Debugf("Saving %s to the tarball", n)
			if err = saver.Add(n); err != nil {
				err = fmt.Errorf("Need to save state, but couldn't: %v", err)
				return
			}

			// Add each detected change
			if _, err = worktree.Add(n); err != nil {
				err = fmt.Errorf("Unable to add %s: %v", n, err)
				return
			}
		}
	}
	return

}

func validBranch(r *git.Repository, hintBranch string) (*plumbing.Reference, string, error) {

	ref, err := r.Head()
	if err != nil {
		return nil, "", fmt.Errorf("no Head: %v", err)
	}

	if ref.Name().IsBranch() {
		if hintBranch != "" {
			if hintBranch == ref.Name().Short() {
				log.Infof("Informed branch is the one used locally")

				return ref, hintBranch, nil

			} else {
				return nil, hintBranch, fmt.Errorf("informed branch (%v) isn't the same as the one used locally (%v), please switch to it first", hintBranch, ref.Name().String())
			}
		} else {
			log.Debugf("Found branch reference name is %v", ref.Name().Short())
			return ref, ref.Name().Short(), nil
		}

	} else {
		return nil, "", fmt.Errorf("no branch set?")
	}
}

func (cmd *RemoteCmd) savePatch() error {

	err := ioutil.WriteFile(cmd.patchPath, cmd.patchData, 0644)

	if err != nil {
		return fmt.Errorf("Couldn't save a local patch file at: %s, because: %v", cmd.patchPath, err)
	}

	return nil
}
