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
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"

	ybconfig "github.com/yourbase/yb/config"
)

var (
	gitExe       string = "git" // UNIX
	gitStatusCmd string = "%s status -s --porcelain"
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
	remotes        []GitRemote
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
}

func (p *RemoteCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

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

	var target BuildTarget

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

	p.repoDir = targetPackage.Path
	workRepo, err := git.PlainOpen(p.repoDir)

	if err != nil {
		log.Errorf("Error opening repository %s: %v", p.repoDir, err)
		return subcommands.ExitFailure
	}

	if !p.goGitStatus && !p.committed {
		// Need to check if `git` binary exists and works
		if runtime.GOOS == "windows" {
			gitExe = "git.exe"
		}
		cmdString := fmt.Sprintf("%s --version", gitExe)
		if err := ExecSilently(cmdString, p.repoDir); err != nil {
			if strings.Contains(err.Error(), "executable file not found in $PATH") {
				log.Warnf("The flag -go-git-status wasn't specified and '%s' wasn't found in PATH", gitExe)
			} else {
				log.Warnf("The flag -go-git-status wasn't specified but calling '%s' gives: %v", cmdString, err)
			}
			log.Warn("Switching to using internal go-git status to detect local changes, it can be slower")
			p.goGitStatus = true
		}
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

	list, err := workRepo.Remotes()

	if err != nil {
		log.Errorf("Error getting remotes for %s: %v", p.repoDir, err)
		return subcommands.ExitFailure
	}

	var repoUrls []string

	for _, r := range list {
		c := r.Config()
		for _, u := range c.URLs {
			repoUrls = append(repoUrls, u)
		}
	}

	project, remote, err := p.fetchProject(repoUrls)

	if err != nil {
		bootErrored()
		log.Errorf("Error fetching project metadata: %v", err)
		return subcommands.ExitFailure
	}

	if !remote.Validate() {
		bootErrored()
		log.Errorf("Unable to pick the correct remote")
		return subcommands.ExitFailure
	}

	if project.Repository == "" {
		projectUrl, err := ybconfig.ManagementUrl(fmt.Sprintf("/organizations/%s/projects/%s", project.OrgSlug, project.Label))
		bootErrored()
		if err != nil {
			log.Errorf("Unable to generate project URL: %v", err)
			return subcommands.ExitFailure
		}

		log.Errorf("Empty repository for project %s, please check your project settings at %s", project.Label, projectUrl)
		return subcommands.ExitFailure
	}

	// First things first:
	// 1. Define correct branch name
	// 2. Try to clone the remote repository using the defined branch
	// 3. Then try to clone it using the default 'master' branch
	// 4. Define common ancestor commit
	// 5. Generate patch file
	//    5.1. Comparing every local commits with the one upstream
	//    5.2. Comparing every unstaged/untracked changes with the one upstream
	//    5.3. Save the patch and compress it
	// 6. Submit build!

	remote.Branch, err = defineBranch(workRepo, p.branch)
	if err != nil {
		bootErrored()
		log.Errorf("Unable to define a branch: %v", err)
		return subcommands.ExitFailure
	}

	if bootProgress != nil {
		fmt.Println()
	}
	ancestorRef, commitCount, branch := fastFindAncestor(workRepo)
	if commitCount == -1 { // Error
		bootErrored()
		return subcommands.ExitFailure
	}
	p.branch = branch
	p.baseCommit = ancestorRef.String()

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

	pGenerationChan := make(chan bool)
	if headCommit.Hash.String() != p.baseCommit {
		if log.CheckIfTerminal() {
			patchProgress = NewProgressSpinner("Generating patch for %d commits", commitCount)
			patchProgress.Start()
		} else {
			log.Infof("Generating patch for %d commits...", commitCount)
		}

		patch, err := ancestorCommit.Patch(headCommit)
		if err != nil {
			patchErrored()
			log.Errorf("Patch generation failed: %v", err)
			return subcommands.ExitFailure
		}
		// This is where the patch is actually generated see #278
		go func(ch chan<- bool) {
			p.patchData = []byte(patch.String())
			ch <- true
		}(pGenerationChan)
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

		saver, err := NewWorktreeSave(targetPackage.Path, headCommit.Hash.String())
		if err != nil {
			patchErrored()
			log.Errorf("%s", err)
		}

		// TODO When go-git worktree.Status() get faster use this instead:
		// There's also the problem detecting CRLF in Windows text/source files
		//if err = worktree.AddGlob("."); err != nil {
		if err = p.traverseChanges(worktree, saver); err != nil {
			log.Errorf("%v", err)
			patchErrored()
			return subcommands.ExitFailure
		}

		resetDone := false
		if len(saver.Files) > 0 {
			// Save them before committing
			if saveFile, err := saver.Save(); err != nil {
				patchErrored()
				log.Errorf("Unable to keep worktree changes, won't commit: %v", err)
				return subcommands.ExitFailure
			} else {
				defer func(s string) {
					if !resetDone {
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

		patch, err := ancestorCommit.Patch(tempCommit)
		if err != nil {
			log.Errorf("Patch generation failed: %v", err)
			patchErrored()
			return subcommands.ExitFailure
		}

		// This is where the patch is actually generated see #278
		p.patchData = []byte(patch.String())

		// Reset back to HEAD
		if err := worktree.Reset(&git.ResetOptions{
			Commit: headCommit.Hash,
		}); err != nil {
			log.Errorf("Unable to reset temporary commit: %v\n    Please try `git reset --hard HEAD~1`", err)
		} else {
			resetDone = true
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

func (p *RemoteCmd) traverseChanges(worktree *git.Worktree, saver *WorktreeSave) (err error) {
	if p.goGitStatus {
		err = p.libTraverseChanges(worktree, saver)
	} else {
		err = p.commandTraverseChanges(worktree, saver)
	}
	return
}

func (p *RemoteCmd) commandTraverseChanges(worktree *git.Worktree, saver *WorktreeSave) (err error) {
	// TODO When go-git worktree.Status() works faster, we'll disable this
	// Get worktree path
	repoDir := worktree.Filesystem.Root()
	cmdString := fmt.Sprintf(gitStatusCmd, gitExe)
	buf := bytes.NewBuffer(nil)
	if err = ExecSilentlyToWriter(cmdString, repoDir, buf); err != nil {
		return fmt.Errorf("When running git status: %v", err)
	}

	if buf.Len() > 0 {
		if p.printStatus {
			fmt.Println()
			fmt.Print(buf.String())
		}
		for {
			if line, err := buf.ReadString('\n'); len(line) > 4 && err == nil { // Line should have at least 4 chars
				line = strings.Trim(line, "\n")
				log.Tracef("Processing git status line:\n%s", line)
				mode := line[:2]
				file := line[3:]
				modUnstagedMap := map[byte]bool{'?': true, 'M': true, 'D': true, 'R': true} // 'R' isn't really found at mode[1], but...

				// Unstaged modifications of any kind
				if modUnstagedMap[mode[1]] {
					if is, _ := IsBinary(path.Join(worktree.Filesystem.Root(), file)); is {
						continue
					}
					log.Tracef("Adding %s to the index", file)
					// Add each detected change
					if _, err = worktree.Add(file); err != nil {
						return fmt.Errorf("Unable to add %s: %v", file, err)
					}

					if mode[1] != 'D' {
						// No need to add deletion to the saver, right?
						log.Tracef("Saving %s to the tarball", file)
						if err = saver.Add(file); err != nil {
							return fmt.Errorf("Need to save state, but couldn't: %v", err)
						}
					}

				}
				// discard len(line) bytes
				//discard := make([]byte, len(line))
				//_, _ = buf.Read(discard)

			} else if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("When running git status: %v", err)
			}
		}
	}
	return
}
func (p *RemoteCmd) libTraverseChanges(worktree *git.Worktree, saver *WorktreeSave) (err error) {
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
		log.Tracef("Checking status %v", n)
		// Deleted (staged removal or not)
		if s.Worktree == git.Deleted {

			if _, err = worktree.Remove(n); err != nil {
				return fmt.Errorf("Unable to remove %s: %v", n, err)
			}
		} else if s.Staging == git.Deleted {
			// Ignore it
		} else if s.Worktree == git.Renamed || s.Staging == git.Renamed {

			log.Tracef("Saving %s to the tarball", n)
			if err = saver.Add(n); err != nil {
				return fmt.Errorf("Need to save state, but couldn't: %v", err)
			}

			if _, err = worktree.Move(s.Extra, n); err != nil {
				return fmt.Errorf("Unable to move %s -> %s: %v", s.Extra, n, err)
			}
		} else {
			if is, _ := IsBinary(path.Join(worktree.Filesystem.Root(), n)); is {
				continue
			}
			log.Tracef("Saving %s to the tarball", n)
			if err = saver.Add(n); err != nil {
				return fmt.Errorf("Need to save state, but couldn't: %v", err)
			}

			// Add each detected change
			if _, err = worktree.Add(n); err != nil {
				return fmt.Errorf("Unable to add %s: %v", n, err)
			}
		}
	}
	return

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

func managementLogUrl(url, org, label string) string {
	wsUrlRegexp := regexp.MustCompile(`^wss?://[^/]+/builds/([0-9a-f-]+)/progress$`)

	if wsUrlRegexp.MatchString(url) {
		submatches := wsUrlRegexp.FindStringSubmatch(url)
		build := ""
		if len(submatches) > 1 {
			build = submatches[1]
		}
		if len(build) == 0 {
			return ""
		}

		u, err := ybconfig.ManagementUrl(fmt.Sprintf("/organizations/%s/projects/%s/builds/%s", org, label, build))
		if err != nil {
			log.Errorf("Unable to generate App Url: %v", err)
		}

		return u
	}
	return ""
}

func defineBranch(r *git.Repository, hintBranch string) (string, error) {

	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("No Head: %v", err)
	}

	if ref.Name().IsBranch() {
		if hintBranch != "" {
			if hintBranch == ref.Name().Short() {
				log.Infof("Informed branch is the one used locally")

				return hintBranch, nil

			} else {
				return hintBranch, fmt.Errorf("Informed branch (%v) isn't the same as the one used locally (%v)", hintBranch, ref.Name().String())
			}
		} else {
			log.Debugf("Found branch reference name is %v", ref.Name().Short())
			return ref.Name().Short(), nil
		}

	} else {
		return "", fmt.Errorf("No branch set?")
	}
}

func (p *RemoteCmd) fetchProject(urls []string) (*Project, GitRemote, error) {
	var empty GitRemote
	v := url.Values{}
	fmt.Println()
	log.Infof("URLs used to search: %s", urls)

	for _, u := range urls {
		rem := NewGitRemote(u)
		// We only support GitHub by now
		// TODO create something more generic
		if rem.Validate() && strings.Contains(rem.String(), "github.com") {
			p.remotes = append(p.remotes, rem)
			v.Add("urls[]", u)
		} else {
			log.Warnf("Invalid remote: '%s', ignoring", u)
		}
	}
	resp, err := postToApi("search/projects", v)

	if err != nil {
		return nil, empty, fmt.Errorf("Couldn't lookup project on api server: %v", err)
	}

	if resp.StatusCode != 200 {
		log.Debugf("Build server returned HTTP Status %d", resp.StatusCode)
		if resp.StatusCode == 203 {
			p.publicRepo = true
		} else if resp.StatusCode == 401 {
			return nil, empty, fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
		} else {
			return nil, empty, fmt.Errorf("This is us, not you, please try again in a few minutes.")
		}
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var project Project
	err = json.Unmarshal(body, &project)
	if err != nil {
		return nil, empty, err
	}

	remote := p.pickRemote(project.Repository)
	if !remote.Validate() {
		return nil, empty, fmt.Errorf("Can't pick a good remote to clone upstream.")
	}

	return &project, remote, nil
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

func (cmd *RemoteCmd) savePatch() error {

	err := ioutil.WriteFile(cmd.patchPath, cmd.patchData, 0644)

	if err != nil {
		return fmt.Errorf("Couldn't save a local patch file at: %s, because: %v", cmd.patchPath, err)
	}

	return nil
}

func (cmd *RemoteCmd) submitBuild(project *Project, tagMap map[string]string) error {

	startTime := time.Now()
	var submitProgress *Progress
	submitErrored := func() {
		if submitProgress != nil {
			submitProgress.Fail()
		}
	}
	if log.CheckIfTerminal() {
		submitProgress = NewProgressSpinner("Submitting remote build")
		submitProgress.Start()
	}

	userToken, err := ybconfig.UserToken()
	if err != nil {
		submitErrored()
		return err
	}

	patchBuffer := bytes.NewBuffer(cmd.patchData)

	if err = CompressBuffer(patchBuffer); err != nil {
		submitErrored()
		return fmt.Errorf("Couldn't compress the patch file: %s", err)
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

	if cmd.noAcceleration {
		formData.Add("no-accel", "True")
	}

	if cmd.disableCache {
		formData.Add("disable-cache", "True")
	}

	if cmd.disableSkipper {
		formData.Add("disable-skipper", "True")
	}

	cliUrl, err := ybconfig.ApiUrl("builds/cli")
	if err != nil {
		log.Debugf("Unable to generate CLI URL: %v", err)
	}
	log.Debugf("Calling backend (%s) with the following values: %v", cliUrl, formData)

	resp, err := postToApi("builds/cli", formData)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		submitErrored()
		return fmt.Errorf("Couldn't read response body: %s", err)
	}
	switch resp.StatusCode {
	case 401:
		submitErrored()
		return fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
	case 403:
		submitErrored()
		if cmd.publicRepo {
			return fmt.Errorf("This should not happen, please open a support inquery with YB")
		} else {
			return fmt.Errorf("Tried to build a private repository of a organization of which you're not part of.")
		}
	case 412:
		// TODO Show helpfull message with App URL to fix GH App installation issue
		submitErrored()
		return fmt.Errorf("Please verify if this specific repo has %s installed", ybconfig.CurrentGHAppUrl())
	case 500:
		submitErrored()
		return fmt.Errorf("Internal server error")
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

	if submitProgress != nil {
		submitProgress.Success()
	}
	endTime := time.Now()
	submitTime := endTime.Sub(startTime)
	log.Infof("Submission finished at %s, taking %s", endTime.Format(TIME_FORMAT), submitTime.Truncate(time.Millisecond))

	startTime = time.Now()
	var remoteProgress *Progress
	remoteErrored := func() {
		if remoteProgress != nil {
			remoteProgress.Fail()
		}
	}
	if log.CheckIfTerminal() {
		remoteProgress = NewProgressSpinner("Setting up remote build")
		remoteProgress.Start()
	}

	if strings.HasPrefix(url, "ws:") || strings.HasPrefix(url, "wss:") {
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), url)
		if err != nil {
			remoteErrored()
			return fmt.Errorf("Cannot connect: %v", err)
		} else {

			defer func() {
				if err = conn.Close(); err != nil {
					log.Debugf("Cannot close: %v", err)
				}
			}()

			buildSuccess := false
			buildSetupFinished := false
			for {
				msg, control, err := wsutil.ReadServerData(conn)
				if err != nil {
					if err != io.EOF {
						log.Debugf("Unstable connection: %v", err)
					} else {
						if buildSuccess {
							log.Infoln("Build Completed!")
						} else {
							log.Errorln("Build failed or the connection was interrupted!")
						}
						log.Infof("Build Log: %v", managementLogUrl(url, project.OrgSlug, project.Label))
						return nil
					}
				} else {
					// TODO This depends on build agent output, try to structure this better
					if control.IsData() && strings.Count(string(msg), "Streaming results from build") > 0 {
						fmt.Println()
					} else if control.IsData() && !buildSetupFinished && len(msg) > 0 {
						buildSetupFinished = true
						if remoteProgress != nil {
							remoteProgress.Success()
						}
						endTime := time.Now()
						setupTime := endTime.Sub(startTime)
						log.Infof("Set up finished at %s, taking %s", endTime.Format(TIME_FORMAT), setupTime.Truncate(time.Millisecond))
						if cmd.publicRepo {
							log.Infof("Building a public repository: '%s'", project.Repository)
						}
						log.Infof("Build Log: %v", managementLogUrl(url, project.OrgSlug, project.Label))
					}
					if !buildSuccess {
						buildSuccess = strings.Count(string(msg), "-- BUILD SUCCEEDED --") > 0
					}
					fmt.Printf("%s", msg)
				}
			}
		}
	} else {
		return fmt.Errorf("Unable to stream build output!")
	}

}
