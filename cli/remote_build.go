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
	"regexp"
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

type RemoteCmd struct {
	target         string
	baseCommit     string
	branch         string
	patchData      []byte
	patchPath      string
	repoDir        string
	noAcceleration bool
	disableCache   bool
	disableSkipper bool
	dryRun         bool
	committed      bool
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
		log.Errorf("Error fetching project metadata: %v", err)
		return subcommands.ExitFailure
	}

	if !remote.Validate() {
		log.Errorf("Unable to pick the correct remote")
		return subcommands.ExitFailure
	}

	if project.Repository == "" {
		projectUrl, err := ybconfig.ManagementUrl(fmt.Sprintf("%s/%s", project.OrgSlug, project.Label))
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
		log.Errorf("Unable to define a branch: %v", err)
		return subcommands.ExitFailure
	}

	ancestorRef, branch := fastFindAncestor(workRepo)
	p.branch = branch

	worktree, err := workRepo.Worktree() // current worktree
	if err != nil {
		log.Errorf("Couldn't get current worktree: %v", err)
		return subcommands.ExitFailure
	}

	head, _ := workRepo.Head()
	headCommit, err := workRepo.CommitObject(head.Hash())
	if err != nil {
		log.Errorf("Couldn't find HEAD commit: %v", err)
		return subcommands.ExitFailure
	}

	// Show timing feedback and start tracking spent time
	startTime := time.Now()
	var patchProgress *Progress
	patchErrored := func() {
		if patchProgress != nil {
			patchProgress.Fail()
		}
	}

	if log.CheckIfTerminal() {
		patchProgress = NewProgressSpinner("Generating patch")
		patchProgress.Start()
	} else {
		log.Info("Generating patch...")
	}
	ancestorCommit, err := workRepo.CommitObject(ancestorRef)
	patch, err := ancestorCommit.Patch(headCommit)
	if err != nil {
		patchErrored()
		log.Errorf("Patch generation failed: %v", err)
		return subcommands.ExitFailure
	}
	p.baseCommit = ancestorCommit.Hash.String()
	// This is where the patch is actually generated see #278
	p.patchData = []byte(patch.String())

	if !p.committed {
		// Apply changes that weren't committed yet
		status, err := worktree.Status()
		if err != nil {
			patchErrored()
			log.Errorf("Couldn't get current worktree status: %v", err)
			return subcommands.ExitFailure
		}

		saver, err := NewWorktreeSave(targetPackage.Path, headCommit.Hash.String())
		if err != nil {
			patchErrored()
			log.Errorf("%s", err)
		}

		for n, s := range status {
			// Deleted (staged removal or not)
			if s.Worktree == git.Deleted || s.Staging == git.Deleted {

				_, err := worktree.Remove(n)
				if err != nil {
					patchErrored()
					log.Errorf("Unable to remove %s: %v", n, err)
					return subcommands.ExitFailure
				}
			} else if s.Worktree == git.Renamed || s.Staging == git.Renamed {

				if err := saver.Add(n); err != nil {
					patchErrored()
					log.Errorf("Need to save state, but couldn't: %v", err)
					return subcommands.ExitFailure
				}

				_, err = worktree.Move(s.Extra, n)
				if err != nil {
					patchErrored()
					log.Errorf("Unable to move %s -> %s: %v", s.Extra, n, err)
					return subcommands.ExitFailure
				}
			} else {
				if err := saver.Add(n); err != nil {
					patchErrored()
					log.Errorf("Need to save state, but couldn't: %v", err)
					return subcommands.ExitFailure
				}

				// Add each detected change
				_, err = worktree.Add(n)
				if err != nil {
					patchErrored()
					log.Errorf("Unable to add %s: %v", n, err)
					return subcommands.ExitFailure
				}
			}
		}

		resetDone := false
		// Save them before committing
		if saveFile, err := saver.Save(); err != nil {
			patchErrored()
			log.Errorf("Unable to keep worktree changes, won't commit: %v", err)
			return subcommands.ExitFailure
		} else {
			defer func(s string) {
				if !resetDone {
					if err := saver.Restore(s); err != nil {
						log.Errorf("Unable to restore kept files at %v: %v\n     Please consider unarchiving yourself that package", saveFile, err)
					} else {
						_ = os.Remove(s)
					}
				} else {
					_ = os.Remove(s)
				}
			}(saveFile)
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

		patch, err = ancestorCommit.Patch(tempCommit)
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

	if p.patchPath != "" {
		if err := p.savePatch(); err != nil {
			if patchProgress != nil {
				fmt.Println()
			}
			log.Warningf("Unable to save copy of generated patch: %v", err)
		}
	}
	// Show feedback: end of patch generation
	endTime := time.Now()
	patchTime := endTime.Sub(startTime)
	if patchProgress != nil {
		patchProgress.Success()
	}
	log.Infof("Patch finished at %s, taking %s", endTime.Format(TIME_FORMAT), patchTime)

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

		u, err := ybconfig.ManagementUrl(fmt.Sprintf("/%s/%s/builds/%s", org, label, build))
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
	for _, u := range urls {
		rem := NewGitRemote(u)
		if rem.Validate() {
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
		if resp.StatusCode == 404 {
			return nil, empty, fmt.Errorf("Is YourBase GitHub App installed?\nPlease check '%v'", ybconfig.CurrentGHAppUrl())
		} else if resp.StatusCode == 401 {
			return nil, empty, fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
		} else if resp.StatusCode == 403 {
			return nil, empty, fmt.Errorf("Access denied, tried to build a repository from a organization that you don't belong to.")
		} else {
			return nil, empty, fmt.Errorf("Error fetching project from API.")
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
		return nil, empty, fmt.Errorf("Can't pick a good remote to clone upstream")
	}

	return &project, remote, nil
}

func (p *RemoteCmd) pickRemote(url string) (remote GitRemote) {

	for _, r := range p.remotes {
		if strings.Contains(url, r.Url) || strings.Contains(r.Url, url) {
			return r
		}
	}
	if len(p.remotes) > 0 {
		remote = p.remotes[0]
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
	case 400:
		submitErrored()
		return fmt.Errorf("Invalid data sent to the YB API")
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
	log.Infof("Submission finished at %s, taking %s", endTime.Format(TIME_FORMAT), submitTime)

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
						log.Errorf("Unstable connection: %v", err)
						// Ignore
						//log.Warnf("can not receive: %v", err)
						//return err
					} else {
						if buildSuccess {
							log.Infoln("Build Completed!")
						} else {
							log.Errorln("Build failed!")
						}
						return nil
					}
				} else {
					// This depends on build agent output
					if control.IsData() && strings.Count(string(msg), "Streaming results from build") > 0 {
						fmt.Println()
					}
					if control.IsData() && !buildSetupFinished && len(msg) > 0 && strings.Count(string(msg), "BUILD DATA PATCHING") > 0 {
						buildSetupFinished = true
						if remoteProgress != nil {
							remoteProgress.Success()
						}
						endTime := time.Now()
						setupTime := endTime.Sub(startTime)
						log.Infof("Set up finished at %s, taking %s", endTime.Format(TIME_FORMAT), setupTime)
						log.Infof("Build Log: %v", managementLogUrl(url, project.OrgSlug, project.Label))
					}
					fmt.Printf("%s", msg)
					buildSuccess = strings.Count(string(msg), "-- BUILD SUCCEEDED --") > 0
				}
			}
		}
	} else {
		return fmt.Errorf("Unable to stream build output!")
	}

}
