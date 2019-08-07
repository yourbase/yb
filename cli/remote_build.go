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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"
	"github.com/sergi/go-diff/diffmatchpatch"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/utils/merkletrie"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"

	ybconfig "github.com/yourbase/yb/config"
)

type RemoteCmd struct {
	target         string
	baseCommit     string
	branch         string
	patchData      []byte
	patchPath      string
	repoDir        string
	noAcceleration bool
	dryRun         bool
	committed      bool
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
	f.BoolVar(&p.noAcceleration, "no-accel", false, "Disable accelaration")
	f.BoolVar(&p.dryRun, "dry-run", false, "Pretend to remote build")
	f.BoolVar(&p.committed, "committed", false, "Only remote build committed changes")
}

func (p *RemoteCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	// Consistent with how the `build` cmd works
	p.target = "default"
	if len(f.Args()) > 0 {
		p.target = f.Args()[0]
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

	manifest := targetPackage.Manifest

	var target BuildTarget

	if len(manifest.BuildTargets) == 0 {
		target = manifest.Build
		if len(target.Commands) == 0 {
			fmt.Printf("Default build command has no steps and no targets described\n")
		}
	} else {
		if _, err := manifest.BuildTarget(p.target); err != nil {
			fmt.Printf("Build target %s specified but it doesn't exist!\n", p.target)
			fmt.Printf("Valid build targets: %s\n", strings.Join(manifest.BuildTargetList(), ", "))
			return subcommands.ExitFailure
		}
	}

	fmt.Printf("Remotely building '%s' ...\n", p.target)
	p.repoDir = targetPackage.Path
	workRepo, err := git.PlainOpen(p.repoDir)

	if err != nil {
		fmt.Printf("Error opening repository %s: %v\n", p.repoDir, err)
		return subcommands.ExitFailure
	}

	list, err := workRepo.Remotes()

	if err != nil {
		fmt.Printf("Error getting remotes for %s: %v\n", p.repoDir, err)
		return subcommands.ExitFailure
	}

	var repoUrls []string

	for _, r := range list {
		c := r.Config()
		for _, u := range c.URLs {
			repoUrls = append(repoUrls, u)
		}
	}

	project, err := fetchProject(repoUrls)

	if err != nil {
		fmt.Printf("Error fetching project metadata: %v\n", err)
		return subcommands.ExitFailure
	}

	if project.Repository == "" {
		projectUrl, err := ybconfig.ManagementUrl(fmt.Sprintf("%s/%s", project.OrgSlug, project.Label))
		if err != nil {
			fmt.Printf("Unable to generate project URL: %v\n", err)
			return subcommands.ExitFailure
		}

		fmt.Printf("Empty repository for project %s, please check your project settings at %s", project.Label, projectUrl)
		return subcommands.ExitFailure
	}

	// First things first:
	// 1. Define correct branch name
	// 2. Try to clone the remote repository using the defined branch
	// 3. Then try to clone it using the default 'master' branch
	// 4. Define baseCommit (informed, validated or found)
	// 5. If branch isn't on remote yet,
	// 6. Define common ancestor commit
	// 7. Generate patch file
	// 8. Submit build!

	foundBranch, err := defineBranch(workRepo, p.branch)

	if err != nil {
		fmt.Printf("Unable to define a branch: %v\n", err)
		return subcommands.ExitFailure
	}

	LOGGER.Debugf("Cloning repo %s, using branch '%s'", project.Repository, foundBranch)

	clonedRepo, err := CloneInMemoryRepo(project.Repository, foundBranch)
	if err != nil {
		fmt.Printf("Unable to clone %v, using branch '%v': %v\n", project.Repository, foundBranch, err)

		clonedRepo, err = CloneInMemoryRepo(project.Repository, "master")
		if err != nil {
			fmt.Printf("Unable to clone %v using the master branch: %v\n", project.Repository, err)
			return subcommands.ExitFailure
		}
		p.branch = "master"
	} else {
		p.branch = foundBranch
	}

	targetSet := commitSet(workRepo)
	if targetSet == nil {
		fmt.Printf("Branch isn't on remote yet and couldn't build a commit set for comparing\n")
		return subcommands.ExitFailure
	}
	commonAncestor := p.findCommonAncestor(clonedRepo, targetSet)

	// Commit workRepo local changes to the cloned in-memory repository
	// First: get them to the in memory cloned Repo

	worktree, err := workRepo.Worktree() // current worktree
	if err != nil {
		fmt.Printf("Couldn't get current worktree: %v\n", err)
		return subcommands.ExitFailure
	}

	clonedWorktree, err := clonedRepo.Worktree() // temporary worktree
	if err != nil {
		fmt.Printf("Couldn't get cloned worktree: %v\n", err)
		return subcommands.ExitFailure
	}

	// Apply changes beetween remote upstream and local repo commits
	latest, err := p.applyCommits(commonAncestor, clonedWorktree, clonedRepo, workRepo)
	if err != nil {
		fmt.Printf("Commits from downstream didn't apply cleanly upstream: %v\n", err)
		return subcommands.ExitFailure
	}

	if !p.committed {
		// Apply changes that wasn't committed yet
		status, err := worktree.Status()
		if err != nil {
			fmt.Printf("Couldn't get current worktree status: %v\n", err)
			return subcommands.ExitFailure
		}

		fmt.Printf("Changes detected by `git status`:\n%s", status)

		for n, s := range status {
			// Deleted (staged removal or not)
			if s.Worktree == git.Deleted || s.Staging == git.Deleted {
				_, err := clonedWorktree.Remove(n)
				if err != nil {
					fmt.Printf("Unable to remove %s from the temporary cloned repository: %v\n", n, err)
					return subcommands.ExitFailure
				}
			} else if s.Worktree == git.Renamed || s.Staging == git.Renamed {
				_, err = clonedWorktree.Move(s.Extra, n)
				if err != nil {
					fmt.Printf("Unable to move %s -> %s in the temporary cloned repository: %v\n", s.Extra, n, err)
					return subcommands.ExitFailure
				}
			} else {
				// Copy contents from the workRepo fs to cloneRepo fs
				originalFile, err := worktree.Filesystem.Open(n)
				if err != nil {
					fmt.Printf("Unable to open %s on the work tree: %v\n", n, err)
					return subcommands.ExitFailure
				}

				newFile, err := clonedWorktree.Filesystem.Create(n)
				if err != nil {
					fmt.Printf("Unable to open %s on the cloned tree: %v\n", n, err)
					return subcommands.ExitFailure
				}

				_, err = io.Copy(newFile, originalFile)
				if err != nil {
					fmt.Printf("Unable to copy %s: %v\n", n, err)
					return subcommands.ExitFailure
				}
				_ = originalFile.Close()
				_ = newFile.Close()

				// Add each detected changed file to the clonedRepo index
				_, err = clonedWorktree.Add(n)
				if err != nil {
					fmt.Printf("Unable to add %s to the temporary cloned repository: %v\n", n, err)
					return subcommands.ExitFailure
				}
			}
		}

		latest, err = clonedWorktree.Commit(
			"YourBase remote build",
			&git.CommitOptions{
				Author: &object.Signature{
					Name:  "YourBase",
					Email: "robot@yourbase.io",
					When:  time.Now(),
				},
			},
		)
		if err != nil {
			fmt.Printf("Commit to temporary cloned repository failed: %v\n", err)
			return subcommands.ExitFailure
		}
	}

	baseCommit, err := clonedRepo.CommitObject(commonAncestor)
	if err != nil {
		fmt.Printf("Commit definition failed: %v\n", err)
		return subcommands.ExitFailure
	}
	tempCommit, err := clonedRepo.CommitObject(latest)
	if err != nil {
		fmt.Printf("Commit definition failed: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Printf("Generating a patch from comparing %s to %s\n", baseCommit.Hash, tempCommit.Hash)

	commitPatch, err := baseCommit.Patch(tempCommit)
	if err != nil {
		fmt.Printf("Patch generation failed: %v\n", err)
		return subcommands.ExitFailure
	}

	p.patchData = []byte(commitPatch.String())

	if p.patchPath != "" {
		if err := savePatch(p); err != nil {
			fmt.Printf("\nUnable to save copy of generated patch: %v\n\n", err)
		} else {
			fmt.Printf("Patch saved at: %v\n", p.patchPath)
		}
	}

	if !p.dryRun {
		err = submitBuild(project, p, target.Tags)

		if err != nil {
			fmt.Printf("Unable to submit build: %v\n", err)
			return subcommands.ExitFailure
		}
	} else {
		fmt.Println("Dry run ended, build not submitted")
	}

	return subcommands.ExitSuccess
}

func postJsonToApi(path string, jsonData []byte) (*http.Response, error) {
	userToken, err := ybconfig.UserToken()

	if err != nil {
		return nil, err
	}

	apiUrl, err := ybconfig.ApiUrl(path)

	if err != nil {
		return nil, fmt.Errorf("Unable to generate API URL: %v", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	return res, err

}

func postToApi(path string, formData url.Values) (*http.Response, error) {
	userToken, err := ybconfig.UserToken()

	if err != nil {
		return nil, fmt.Errorf("Couldn't get user token: %v", err)
	}

	apiUrl, err := ybconfig.ApiUrl(path)
	if err != nil {
		return nil, fmt.Errorf("Couldn't determine API URL: %v", err)
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("Couldn't make API call: %v", err)
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func defineBranch(r *git.Repository, hintBranch string) (string, error) {

	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("No Head: %v\n", err)
	}

	if ref.Name().IsBranch() {
		if hintBranch != "" {
			if hintBranch == ref.Name().Short() {
				fmt.Printf("Informed branch is the one used locally")

				return hintBranch, nil

			} else {
				return hintBranch, fmt.Errorf("Informed branch (%v) isn't the same as the one used locally (%v)", hintBranch, ref.Name().String())
			}
		} else {
			LOGGER.Debugf("Found branch reference name is %v", ref.Name().Short())
			return ref.Name().Short(), nil
		}

	} else {
		return "", fmt.Errorf("No branch set?")
	}
}

func defineCommit(r *git.Repository, commit string) (string, error) {

	if commit == "" {
		ref, err := r.Head()
		if err != nil {
			return "", fmt.Errorf("No Head: %v", err)
		}

		return ref.Hash().String(), nil
	}

	_, err := r.CommitObject(plumbing.NewHash(commit))

	if err == plumbing.ErrObjectNotFound {
		return "", fmt.Errorf("No commit %s found in the current dir git worktree: %v", commit, err)
	}

	return commit, nil
}

func fetchProject(urls []string) (*Project, error) {
	v := url.Values{}
	for _, u := range urls {
		fmt.Printf("Adding remote URL %s to search...\n", u)
		v.Add("urls[]", u)
	}
	resp, err := postToApi("search/projects", v)

	if err != nil {
		return nil, fmt.Errorf("Couldn't lookup project on api server: %v", err)
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("Couldn't find the project, make sure you have created one whose repository URL matches one of these repository's remotes.")
		} else if resp.StatusCode == 401 {
			return nil, fmt.Errorf("You don't have access to remotely build these repositories: %v", urls)
		} else {
			return nil, fmt.Errorf("Error fetching project from API, can't build remotely.")
		}
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var project Project
	err = json.Unmarshal(body, &project)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func savePatch(cmd *RemoteCmd) error {

	err := ioutil.WriteFile(cmd.patchPath, cmd.patchData, 0644)

	if err != nil {
		return fmt.Errorf("Couldn't save a local patch file at: %s, because: %v", cmd.patchPath, err)
	}

	return nil
}

func (cmd *RemoteCmd) applyCommits(ancestor plumbing.Hash, w *git.Worktree, dstRepo, srcRepo *git.Repository) (plumbing.Hash, error) {
	if w == nil || srcRepo == nil || dstRepo == nil {
		return plumbing.ZeroHash, fmt.Errorf("Needs a worktree and two repositories")
	}

	dstCommit, _ := dstRepo.CommitObject(ancestor)

	var latest plumbing.Hash

	commitIter, _ := srcRepo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime})
	err := commitIter.ForEach(func(srcCommit *object.Commit) error {
		// Apply each one
		if err := applyDiffs(srcCommit, dstCommit, w); err != nil {
			return err
		}

		_, err := w.Add(".") // Stages everything respecting .gitignore
		if err != nil {
			return err
		}

		latest, err = w.Commit("YourBase intermediate commit",
			&git.CommitOptions{
				Author: &object.Signature{
					Name:  "YourBase",
					Email: "robot@yourbase.io",
					When:  time.Now(),
				},
			},
		)
		if err != nil {
			return err
		}
		if srcCommit.Hash.String() == ancestor.String() {
			// stop now, at the ancestor commit
			return storer.ErrStop
		}
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return plumbing.ZeroHash, fmt.Errorf("Unable to stage/commit intermediate changes: %v\n", err)
	}

	return latest, nil
}

func applyDiffs(src, dst *object.Commit, w *git.Worktree) error {
	srcTree, _ := src.Tree()
	dstTree, _ := dst.Tree()

	changes, err := srcTree.Diff(dstTree)
	if err != nil {
		return err
	}

	for _, change := range changes {
		from, to, err := change.Files()
		if err != nil {
			return err
		}
		changeAction, err := change.Action()
		if err != nil {
			return err
		}

		switch changeAction {
		case merkletrie.Delete:
			_, _ = w.Remove(from.Name)
		default:
			if from != nil && to != nil {
				if from.Name != to.Name {
					_, _ = w.Move(from.Name, to.Name)
				}
			}

			contents, err := to.Contents()
			if err != nil {
				return err
			}

			newFile, err := w.Filesystem.Create(to.Name) // Truncates it
			if err != nil {
				return err
			}
			_, err = newFile.Write([]byte(contents))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ApplyPatch(patch, from string) (to string, err error) {
	dmp := diffmatchpatch.New()
	patches, err := dmp.PatchFromText(patch)
	if err != nil {
		return
	}

	to, checks := dmp.PatchApply(patches, from)
	for i, check := range checks {
		if !check {
			return "", fmt.Errorf("Patch hunk #%d failed", i)
		}
	}

	return
}

// TODO use ApplyPatch to substitute calling the `patch -p1 -i` commmand on the Build Agent
func UnifiedPatchApply(patch, baseDir string) (err error) {
	// Detect in the patch string (unified) which files where affected and how
	//   and execute a os.Remove, os.Move and change of contents if it is only a modification
	//   maybe change file Mode too
	//   NOTE: this may be only useful on Windows (portable code)
	return
}

func (cmd *RemoteCmd) findCommonAncestor(r *git.Repository, commits map[string]bool) plumbing.Hash {
	if commits[cmd.baseCommit] {
		// User requested specific commit
		commit, err := r.CommitObject(plumbing.NewHash(cmd.baseCommit))
		if err != nil {
			fmt.Printf("Couldn't find %s commit on remote cloned repository: %v\n", cmd.baseCommit, err)
		} else {
			fmt.Printf("Decided commit: %s\n", commit.Hash)
			return commit.Hash
		}
	}

	ref, err := r.Head()
	if err != nil {
		fmt.Printf("No Head: %v\n", err)
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

	LOGGER.Debugf("Common commit hash: %s\n", commonCommit.Hash)
	fmt.Printf("Decided commit: %s\n", commonCommit.Hash)
	cmd.baseCommit = commonCommit.Hash.String()

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
	commitIter, _ := r.Log(&git.LogOptions{From: commit.Hash, Order: git.LogOrderCommitterTime})
	hashSet := make(map[string]bool)

	err = commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		hashSet[hash] = true
		return nil
	})

	return hashSet

}

func submitBuild(project *Project, cmd *RemoteCmd, tagMap map[string]string) error {

	userToken, err := ybconfig.UserToken()

	if err != nil {
		return err
	}

	patchBuffer := bytes.NewBuffer(cmd.patchData)

	if err = CompressBuffer(patchBuffer); err != nil {
		return fmt.Errorf("Couldn't compress the patch file: \n    %s\n", err)
	}

	patchEncoded := base64.StdEncoding.EncodeToString(patchBuffer.Bytes())

	formData := url.Values{
		"project_id": {strconv.Itoa(project.Id)},
		"repository": {project.Repository},
		"api_key":    {userToken},
		"target":     {cmd.target},
		"patch_data": {patchEncoded},
		"commit":     {cmd.baseCommit},
		"branch":     {cmd.branch},
	}

	tags := make([]string, 0)
	for k, v := range tagMap {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	for _, tag := range tags {
		formData.Add("tags[]", tag)
	}

	cliUrl, err := ybconfig.ApiUrl("builds/cli")
	if err != nil {
		LOGGER.Debugf("Unable to generate CLI URL: %v\n", err)
	}
	LOGGER.Debugf("Calling backend (%s) with the following values: %v\n", cliUrl, formData)

	resp, err := postToApi("builds/cli", formData)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//fmt.Println(resp.Body)
	if err != nil {
		return fmt.Errorf("Couldn't read response body: %s\n", err)
	}

	response := string(body)

	//Process simple response from the API
	response = strings.ReplaceAll(response, "\"", "")

	url := ""
	if strings.Count(response, "\n") > 0 {
		url = strings.Split(response, "\n")[0]
	} else {
		url = response
	}

	if strings.HasPrefix(url, "ws:") || strings.HasPrefix(url, "wss:") {
		fmt.Printf("Streaming build output from %s\n", url)
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), url)
		if err != nil {
			return fmt.Errorf("Can not connect: %v\n", err)
		} else {
			for {
				msg, _, err := wsutil.ReadServerData(conn)
				if err != nil {
					if err != io.EOF {
						// Ignore
						//fmt.Printf("can not receive: %v\n", err)
						//return err
					} else {
						fmt.Println("\n\n\nBuild Completed!")
						return nil
					}
				} else {
					fmt.Printf("%s", msg)
				}
			}

			err = conn.Close()
			if err != nil {
				fmt.Printf("Can not close: %v\n", err)
			} else {
				fmt.Printf("Closed\n")
			}
		}
	} else {
		return fmt.Errorf("Unable to stream build output!")
	}

	return nil

}
