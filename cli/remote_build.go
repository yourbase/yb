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
	"runtime"
	"strconv"
	"strings"
	"time"

	ggit "gg-scm.io/pkg/git"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"
	ybconfig "github.com/yourbase/yb/config"
	"github.com/yourbase/yb/plumbing"
	"github.com/yourbase/yb/types"
	"gopkg.in/src-d/go-git.v4"
	gitplumbing "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"zombiezen.com/go/log"
)

var (
	gitExe       string = "git" // UNIX
	gitStatusCmd string = "%s status --porcelain"
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
	backupWorktree bool
	remotes        []*url.URL
}

func (*RemoteCmd) Name() string     { return "remotebuild" }
func (*RemoteCmd) Synopsis() string { return "Build remotely" }
func (*RemoteCmd) Usage() string {
	return `Usage: remotebuild [TARGET] [OPTIONS]
Build remotely using YourBase infrastructure. Defaults to the "default" target.
`
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
	f.BoolVar(&p.backupWorktree, "backup-worktree", false, "Saves uncommitted work into a tarball")
}

func (p *RemoteCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	// Consistent with how the `build` cmd works
	p.target = "default"
	if len(f.Args()) > 0 {
		p.target = f.Args()[0]
	}

	targetPackage, err := GetTargetPackage()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	manifest := targetPackage.Manifest

	var target *types.BuildTarget

	if len(manifest.BuildTargets) == 0 {
		target = manifest.Build
		if len(target.Commands) == 0 {
			log.Errorf(ctx, "Default build command has no steps and no targets described")
			return subcommands.ExitFailure
		}
	} else {
		var err error
		target, err = manifest.BuildTarget(p.target)
		if err != nil {
			log.Errorf(ctx, "Build target %s specified but it doesn't exist!", p.target)
			log.Infof(ctx, "Valid build targets: %s", strings.Join(manifest.BuildTargetList(), ", "))
			return subcommands.ExitFailure
		}
	}

	p.repoDir = targetPackage.Path
	workRepo, err := git.PlainOpen(p.repoDir)

	if err != nil {
		log.Errorf(ctx, "Error opening repository %s: %v", p.repoDir, err)
		return subcommands.ExitFailure
	}

	if !p.goGitStatus && !p.committed {
		// Need to check if `git` binary exists and works
		if runtime.GOOS == "windows" {
			gitExe = "git.exe"
		}
		cmdString := fmt.Sprintf("%s --version", gitExe)
		if err := plumbing.ExecSilently(cmdString, p.repoDir); err != nil {
			if strings.Contains(err.Error(), "executable file not found in $PATH") {
				log.Warnf(ctx, "The flag -go-git-status wasn't specified and '%s' wasn't found in PATH", gitExe)
			} else {
				log.Warnf(ctx, "The flag -go-git-status wasn't specified but calling '%s' gives: %v", cmdString, err)
			}
			log.Warnf(ctx, "Switching to using internal go-git status to detect local changes, it can be slower")
			p.goGitStatus = true
		}
	}

	// Show timing feedback and start tracking spent time
	startTime := time.Now()

	log.Infof(ctx, "Bootstrapping...")

	list, err := workRepo.Remotes()

	if err != nil {
		log.Errorf(ctx, "Error getting remotes for %s: %v", p.repoDir, err)
		return subcommands.ExitFailure
	}

	var repoUrls []string

	for _, r := range list {
		c := r.Config()
		repoUrls = append(repoUrls, c.URLs...)
	}

	project, err := p.fetchProject(ctx, repoUrls)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		return subcommands.ExitFailure
	}

	if project.Repository == "" {
		projectURL, err := ybconfig.UIURL(fmt.Sprintf("%s/%s", project.OrgSlug, project.Label))
		if err != nil {
			log.Errorf(ctx, "Unable to generate project URL: %v", err)
			return subcommands.ExitFailure
		}

		log.Errorf(ctx, "Empty repository for project %s, please check your project settings at %s", project.Label, projectURL)
		return subcommands.ExitFailure
	}

	// First things first:
	// 1. Define correct branch name
	// 2. Define common ancestor commit
	// 3. Generate patch file
	//    3.1. Comparing every local commits with the one upstream
	//    3.2. Comparing every unstaged/untracked changes with the one upstream
	//    3.3. Save the patch and compress it
	// 4. Submit build!

	ancestorRef, commitCount, branch := fastFindAncestor(ctx, workRepo)
	if commitCount == -1 { // Error
		return subcommands.ExitFailure
	}
	p.branch = branch
	p.baseCommit = ancestorRef.String()

	head, err := workRepo.Head()
	if err != nil {
		log.Errorf(ctx, "Couldn't find HEAD commit: %v", err)
		return subcommands.ExitFailure
	}
	headCommit, err := workRepo.CommitObject(head.Hash())
	if err != nil {
		log.Errorf(ctx, "Couldn't find HEAD commit: %v", err)
		return subcommands.ExitFailure
	}
	ancestorCommit, err := workRepo.CommitObject(ancestorRef)
	if err != nil {
		log.Errorf(ctx, "Couldn't find merge-base commit: %v", err)
		return subcommands.ExitFailure
	}

	// Show feedback: end of bootstrap
	endTime := time.Now()
	bootTime := endTime.Sub(startTime)
	log.Infof(ctx, "Bootstrap finished at %s, taking %s", endTime.Format(TIME_FORMAT), bootTime.Truncate(time.Millisecond))

	// Process patches
	startTime = time.Now()
	pGenerationChan := make(chan bool)
	if p.committed && headCommit.Hash.String() != p.baseCommit {
		log.Infof(ctx, "Generating patch for %d commits...", commitCount)

		patch, err := ancestorCommit.Patch(headCommit)
		if err != nil {
			log.Errorf(ctx, "Patch generation failed: %v", err)
			return subcommands.ExitFailure
		}
		// This is where the patch is actually generated see #278
		go func(ch chan<- bool) {
			log.Debugf(ctx, "Starting the actual patch generation...")
			p.patchData = []byte(patch.String())
			log.Debugf(ctx, "Patch generation finished, only committed changes")
			ch <- true
		}(pGenerationChan)
	} else if !p.committed {
		// Apply changes that weren't committed yet
		worktree, err := workRepo.Worktree() // current worktree
		if err != nil {
			log.Errorf(ctx, "Couldn't get current worktree: %v", err)
			return subcommands.ExitFailure
		}

		log.Infof(ctx, "Generating patch for local changes...")

		log.Debugf(ctx, "Start backing up the worktree-save")
		saver, err := types.NewWorktreeSave(targetPackage.Path, headCommit.Hash.String(), p.backupWorktree)
		if err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}

		// TODO When go-git worktree.Status() get faster use this instead:
		// There's also the problem detecting CRLF in Windows text/source files
		//if err = worktree.AddGlob("."); err != nil {
		skippedBinaries, err := p.traverseChanges(ctx, worktree, saver)
		if err != nil {
			log.Errorf(ctx, "%v", err)
			return subcommands.ExitFailure
		}
		if len(skippedBinaries) > 0 {
			log.Infof(ctx, "Skipped binaries:\n  %s", strings.Join(skippedBinaries, "\n  "))
		}

		resetDone := false
		if p.backupWorktree && len(saver.Files) > 0 {
			log.Debugf(ctx, "Saving a tarball  with all the worktree changes made")
			// Save them before committing
			if saveFile, err := saver.Save(); err != nil {
				log.Errorf(ctx, "Unable to keep worktree changes, won't commit: %v", err)
				return subcommands.ExitFailure
			} else {
				defer func(s string) {
					if !resetDone {
						log.Debugf(ctx, "Reset failed, restoring the tarball")
						if err := saver.Restore(s); err != nil {
							log.Errorf(ctx, "Unable to restore kept files at %v: %v\n     Please consider unarchiving that package", saveFile, err)
						} else {
							_ = os.Remove(s)
						}
					} else {
						_ = os.Remove(s)
					}
				}(saveFile)
			}
		}

		log.Debugf(ctx, "Committing temporary changes")
		latest, err := commitTempChanges(worktree, headCommit)
		if err != nil {
			log.Errorf(ctx, "Commit to temporary cloned repository failed: %v", err)
			return subcommands.ExitFailure
		}

		tempCommit, err := workRepo.CommitObject(latest)
		if err != nil {
			log.Errorf(ctx, "Can't find commit '%v': %v", latest, err)
			return subcommands.ExitFailure
		}

		log.Debugf(ctx, "Starting the actual patch generation...")
		patch, err := ancestorCommit.Patch(tempCommit)
		if err != nil {
			log.Errorf(ctx, "Patch generation failed: %v", err)
			return subcommands.ExitFailure
		}

		// This is where the patch is actually generated see #278
		p.patchData = []byte(patch.String())
		log.Debugf(ctx, "Actual patch generation finished")

		log.Debugf(ctx, "Reseting worktree to previous state...")
		// Reset back to HEAD
		if err := worktree.Reset(&git.ResetOptions{
			Commit: headCommit.Hash,
		}); err != nil {
			log.Errorf(ctx, "Unable to reset temporary commit: %v\n    Please try `git reset --hard HEAD~1`", err)
		} else {
			resetDone = true
		}
		log.Debugf(ctx, "Worktree reset done.")

	}

	// Show feedback: end of patch generation
	endTime = time.Now()
	patchTime := endTime.Sub(startTime)
	log.Infof(ctx, "Patch finished at %s, taking %s", endTime.Format(TIME_FORMAT), patchTime.Truncate(time.Millisecond))
	if len(p.patchPath) > 0 && len(p.patchData) > 0 {
		if err := p.savePatch(); err != nil {
			log.Warnf(ctx, "Unable to save copy of generated patch: %v", err)
		}
	}

	if !p.dryRun {
		err = p.submitBuild(ctx, project, target.Tags)

		if err != nil {
			log.Errorf(ctx, "Unable to submit build: %v", err)
			return subcommands.ExitFailure
		}
	} else {
		log.Infof(ctx, "Dry run ended, build not submitted")
	}

	return subcommands.ExitSuccess
}

func commitTempChanges(w *git.Worktree, c *object.Commit) (latest gitplumbing.Hash, err error) {
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

func (p *RemoteCmd) traverseChanges(ctx context.Context, worktree *git.Worktree, saver *types.WorktreeSave) (skipped []string, err error) {
	if p.goGitStatus {
		log.Debugf(ctx, "Decided to use Go-Git")
		skipped, err = p.libTraverseChanges(ctx, worktree, saver)
	} else {
		log.Debugf(ctx, "Decided to use `%s`", gitStatusCmd)
		skipped, err = p.commandTraverseChanges(ctx, worktree, saver)
	}
	return
}

func shouldSkip(ctx context.Context, file string, worktree *git.Worktree) bool {
	filePath := path.Join(worktree.Filesystem.Root(), file)
	fi, err := os.Stat(filePath)
	if err != nil {
		return true
	}

	if fi.IsDir() {
		log.Debugf(ctx, "Added a dir, checking it's contents: %s", file)
		f, err := os.Open(filePath)
		if err != nil {
			return true
		}
		dir, err := f.Readdir(0)
		log.Debugf(ctx, "Ls dir %s", filePath)
		for _, f := range dir {
			child := path.Join(file, f.Name())
			log.Debugf(ctx, "Shoud skip child '%s'?", child)
			if shouldSkip(ctx, child, worktree) {
				continue
			} else {
				worktree.Add(child)
			}
		}
		return true
	} else {
		log.Debugf(ctx, "Should skip '%s'?", filePath)
		if is, _ := plumbing.IsBinary(filePath); is {
			return true
		}
	}
	return false
}

func (p *RemoteCmd) commandTraverseChanges(ctx context.Context, worktree *git.Worktree, saver *types.WorktreeSave) (skipped []string, err error) {
	// TODO When go-git worktree.Status() works faster, we'll disable this
	// Get worktree path
	repoDir := worktree.Filesystem.Root()
	cmdString := fmt.Sprintf(gitStatusCmd, gitExe)
	buf := bytes.NewBuffer(nil)
	log.Debugf(ctx, "Executing '%v'...", cmdString)
	if err = plumbing.ExecSilentlyToWriter(cmdString, repoDir, buf); err != nil {
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
				log.Debugf(ctx, "Processing git status line:\n%s", line)
				mode := line[:2]
				file := line[3:]
				modUnstagedMap := map[byte]bool{'?': true, 'M': true, 'D': true, 'R': true} // 'R' isn't really found at mode[1], but...

				// Unstaged modifications of any kind
				if modUnstagedMap[mode[1]] {
					if shouldSkip(ctx, file, worktree) {
						skipped = append(skipped, file)
						continue
					}
					log.Debugf(ctx, "Adding %s to the index", file)
					// Add each detected change
					if _, err = worktree.Add(file); err != nil {
						return skipped, fmt.Errorf("Unable to add %s: %v", file, err)
					}

					if mode[1] != 'D' {
						// No need to add deletion to the saver, right?
						log.Debugf(ctx, "Saving %s to the tarball", file)
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
func (p *RemoteCmd) libTraverseChanges(ctx context.Context, worktree *git.Worktree, saver *types.WorktreeSave) (skipped []string, err error) {
	// This could get real slow, check https://github.com/src-d/go-git/issues/844
	status, err := worktree.Status()
	if err != nil {
		log.Errorf(ctx, "Couldn't get current worktree status: %v", err)
		return
	}

	if p.printStatus {
		fmt.Println()
		fmt.Print(status.String())
	}
	for n, s := range status {
		log.Debugf(ctx, "Checking status %v", n)
		// Deleted (staged removal or not)
		if s.Worktree == git.Deleted {

			if _, err = worktree.Remove(n); err != nil {
				err = fmt.Errorf("Unable to remove %s: %v", n, err)
				return
			}
		} else if s.Staging == git.Deleted {
			// Ignore it
		} else if s.Worktree == git.Renamed || s.Staging == git.Renamed {

			log.Debugf(ctx, "Saving %s to the tarball", n)
			if err = saver.Add(n); err != nil {
				err = fmt.Errorf("Need to save state, but couldn't: %v", err)
				return
			}

			if _, err = worktree.Move(s.Extra, n); err != nil {
				err = fmt.Errorf("Unable to move %s -> %s: %v", s.Extra, n, err)
				return
			}
		} else {
			if shouldSkip(ctx, n, worktree) {
				skipped = append(skipped, n)
				continue
			}
			log.Debugf(ctx, "Saving %s to the tarball", n)
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

func postToApi(path string, formData url.Values) (*http.Response, error) {
	userToken, err := ybconfig.UserToken()

	if err != nil {
		return nil, fmt.Errorf("Couldn't get user token: %v", err)
	}

	apiURL, err := ybconfig.APIURL(path)
	if err != nil {
		return nil, fmt.Errorf("Couldn't determine API URL: %v", err)
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
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

// buildIDFromLogURL returns the build ID in a build log WebSocket URL.
//
// TODO(ch2570): This should come from the API.
func buildIDFromLogURL(u *url.URL) (string, error) {
	// Pattern is /builds/ID/progress
	const prefix = "/builds/"
	const suffix = "/progress"
	if !strings.HasPrefix(u.Path, prefix) || !strings.HasSuffix(u.Path, suffix) {
		return "", fmt.Errorf("build ID for %v: unrecognized path", u)
	}
	id := u.Path[len(prefix) : len(u.Path)-len(suffix)]
	if strings.ContainsRune(id, '/') {
		return "", fmt.Errorf("build ID for %v: unrecognized path", u)
	}
	return id, nil
}

func (p *RemoteCmd) fetchProject(ctx context.Context, urls []string) (*types.Project, error) {
	v := url.Values{}
	fmt.Println()
	log.Infof(ctx, "URLs used to search: %s", urls)

	for _, u := range urls {
		rem, err := ggit.ParseURL(u)
		if err != nil {
			log.Warnf(ctx, "Invalid remote %s (%v), ignoring", u, err)
			continue
		}
		// We only support GitHub by now
		// TODO create something more generic
		if rem.Host != "github.com" {
			log.Warnf(ctx, "Ignoring remote %s (only github.com supported)", u)
			continue
		}
		p.remotes = append(p.remotes, rem)
		v.Add("urls[]", u)
	}
	resp, err := postToApi("search/projects", v)
	if err != nil {
		return nil, fmt.Errorf("Couldn't lookup project on api server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debugf(ctx, "Build server returned HTTP Status %d", resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusNonAuthoritativeInfo:
			p.publicRepo = true
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
		case http.StatusPreconditionFailed, http.StatusNotFound:
			return nil, fmt.Errorf("Please verify if this private repository has %s installed.", ybconfig.GitHubAppURL())
		default:
			return nil, fmt.Errorf("This is us, not you, please try again in a few minutes.")
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var project types.Project
	err = json.Unmarshal(body, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (cmd *RemoteCmd) savePatch() error {

	err := ioutil.WriteFile(cmd.patchPath, cmd.patchData, 0644)

	if err != nil {
		return fmt.Errorf("Couldn't save a local patch file at: %s, because: %v", cmd.patchPath, err)
	}

	return nil
}

func (cmd *RemoteCmd) submitBuild(ctx context.Context, project *types.Project, tagMap map[string]string) error {

	startTime := time.Now()

	userToken, err := ybconfig.UserToken()
	if err != nil {
		return err
	}

	patchBuffer := bytes.NewBuffer(cmd.patchData)

	if err = plumbing.CompressBuffer(patchBuffer); err != nil {
		return fmt.Errorf("Couldn't compress the patch file: %s", err)
	}

	patchEncoded := base64.StdEncoding.EncodeToString(patchBuffer.Bytes())

	formData := url.Values{
		"project_id": {strconv.Itoa(project.ID)},
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

	resp, err := postToApi("builds/cli", formData)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Couldn't read response body: %s", err)
	}
	switch resp.StatusCode {
	case 401:
		return fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
	case 403:
		if cmd.publicRepo {
			return fmt.Errorf("This should not happen, please open a support inquery with YB")
		} else {
			return fmt.Errorf("Tried to build a private repository of a organization of which you're not part of.")
		}
	case 412:
		// TODO Show helpful message with App URL to fix GH App installation issue
		return fmt.Errorf("Please verify if this specific repo has %s installed", ybconfig.GitHubAppURL())
	case 500:
		return fmt.Errorf("Internal server error")
	}
	//Process simple response from the API
	body = bytes.ReplaceAll(body, []byte(`"`), nil)
	if i := bytes.IndexByte(body, '\n'); i != -1 {
		body = body[:i]
	}
	logURL, err := url.Parse(string(body))
	if err != nil {
		return fmt.Errorf("server response: parse log URL: %w", err)
	}
	if logURL.Scheme != "ws" && logURL.Scheme != "wss" {
		return fmt.Errorf("server response: parse log URL: unhandled scheme %q", logURL.Scheme)
	}
	// Construct UI URL to present to the user.
	// Fine to proceed in the face of errors: this is displayed as a fallback if
	// other things fail.
	uiURL := ""
	if id, err := buildIDFromLogURL(logURL); err != nil {
		log.Warnf(ctx, "Could not construct build link: %v", err)
	} else {
		uiURL, err = ybconfig.UIURL("/" + project.OrgSlug + "/" + project.Label + "/builds/" + id)
		if err != nil {
			log.Warnf(ctx, "Could not construct build link: %v", err)
		}
	}

	endTime := time.Now()
	submitTime := endTime.Sub(startTime)
	log.Infof(ctx, "Submission finished at %s, taking %s", endTime.Format(TIME_FORMAT), submitTime.Truncate(time.Millisecond))

	startTime = time.Now()

	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), logURL.String())
	if err != nil {
		return fmt.Errorf("Cannot connect: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Debugf(ctx, "Cannot close: %v", err)
		}
	}()

	buildSuccess := false
	buildSetupFinished := false

	for {
		msg, control, err := wsutil.ReadServerData(conn)
		if err != nil {
			if err != io.EOF {
				log.Debugf(ctx, "Unstable connection: %v", err)
			} else {
				if buildSuccess {
					log.Infof(ctx, "Build Completed!")
				} else {
					log.Errorf(ctx, "Build failed or the connection was interrupted!")
				}
				if uiURL != "" {
					log.Infof(ctx, "Build Log: %s", uiURL)
				}
				return nil
			}
		} else {
			// TODO This depends on build agent output, try to structure this better
			if control.IsData() && strings.Count(string(msg), "Streaming results from build") > 0 {
				fmt.Println()
			} else if control.IsData() && !buildSetupFinished && len(msg) > 0 {
				buildSetupFinished = true
				endTime := time.Now()
				setupTime := endTime.Sub(startTime)
				log.Infof(ctx, "Set up finished at %s, taking %s", endTime.Format(TIME_FORMAT), setupTime.Truncate(time.Millisecond))
				if cmd.publicRepo {
					log.Infof(ctx, "Building a public repository: '%s'", project.Repository)
				}
				if uiURL != "" {
					log.Infof(ctx, "Build Log: %s", uiURL)
				}
			}
			if !buildSuccess {
				buildSuccess = strings.Count(string(msg), "-- BUILD SUCCEEDED --") > 0
			}
			os.Stdout.Write(msg)
		}
	}
}
