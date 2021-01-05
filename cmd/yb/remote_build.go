package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ggit "gg-scm.io/pkg/git"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/archiver"
	"github.com/spf13/cobra"
	"github.com/ulikunitz/xz"
	"github.com/yourbase/commons/http/headers"
	"github.com/yourbase/yb"
	ybconfig "github.com/yourbase/yb/internal/config"
	"gopkg.in/src-d/go-git.v4"
	gitplumbing "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"zombiezen.com/go/log"
)

type remoteCmd struct {
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
	publicRepo     bool
	backupWorktree bool
	remotes        []*url.URL
}

func newRemoteCmd() *cobra.Command {
	p := new(remoteCmd)
	c := &cobra.Command{
		Use:   "remotebuild [options] [TARGET]",
		Short: "Build a target remotely",
		Long: `Builds a target using YourBase infrastructure. If no argument is given, ` +
			`uses the target named "` + yb.DefaultTarget + `", if there is one.` +
			"\n\n" +
			`yb remotebuild will search for the .yourbase.yml file in the current ` +
			`directory and its parent directories. The target's commands will be run ` +
			`in the directory the .yourbase.yml file appears in.`,
		Args:                  cobra.MaximumNArgs(1),
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			p.target = yb.DefaultTarget
			if len(args) > 0 {
				p.target = args[0]
			}
			return p.run(cmd.Context())
		},
		ValidArgsFunction: func(cc *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return autocompleteTargetName(toComplete)
		},
	}
	c.Flags().StringVar(&p.baseCommit, "base-commit", "", "Base commit hash as common ancestor")
	c.Flags().StringVar(&p.branch, "branch", "", "Branch name")
	c.Flags().StringVar(&p.patchPath, "patch-path", "", "Path to save the patch")
	c.Flags().BoolVar(&p.noAcceleration, "no-accel", false, "Disable acceleration")
	c.Flags().BoolVar(&p.disableCache, "disable-cache", false, "Disable cache acceleration")
	c.Flags().BoolVar(&p.disableSkipper, "disable-skipper", false, "Disable skipping steps acceleration")
	c.Flags().BoolVarP(&p.dryRun, "dry-run", "n", false, "Pretend to remote build")
	c.Flags().BoolVar(&p.committed, "committed", false, "Only remote build committed changes")
	c.Flags().BoolVar(&p.backupWorktree, "backup-worktree", false, "Saves uncommitted work into a tarball")
	return c
}

func (p *remoteCmd) run(ctx context.Context) error {
	targetPackage, _, err := findPackage()
	if err != nil {
		return err
	}

	target := targetPackage.Targets[p.target]
	if target == nil {
		return fmt.Errorf("%s: no such target (found: %s)", p.target, strings.Join(listTargetNames(targetPackage.Targets), ", "))
	}

	p.repoDir = targetPackage.Path
	workRepo, err := git.PlainOpen(p.repoDir)

	if err != nil {
		return fmt.Errorf("opening repository %s: %w", p.repoDir, err)
	}

	g, err := ggit.New(ggit.Options{
		Dir: targetPackage.Path,
		LogHook: func(ctx context.Context, args []string) {
			log.Debugf(ctx, "running git %s", strings.Join(args, " "))
		},
	})
	if err != nil {
		return err
	}

	// Show timing feedback and start tracking spent time
	startTime := time.Now()

	log.Infof(ctx, "Bootstrapping...")

	list, err := workRepo.Remotes()

	if err != nil {
		return fmt.Errorf("getting remotes for %s: %w", p.repoDir, err)
	}

	var repoUrls []string

	for _, r := range list {
		c := r.Config()
		repoUrls = append(repoUrls, c.URLs...)
	}

	project, err := p.fetchProject(ctx, repoUrls)
	if err != nil {
		return err
	}

	if project.Repository == "" {
		projectURL, err := ybconfig.UIURL(fmt.Sprintf("%s/%s", project.OrgSlug, project.Label))
		if err != nil {
			return err
		}

		return fmt.Errorf("empty repository for project %s. Please check your project settings at %s", project.Label, projectURL)
	}

	// First things first:
	// 1. Define correct branch name
	// 2. Define common ancestor commit
	// 3. Generate patch file
	//    3.1. Comparing every local commits with the one upstream
	//    3.2. Comparing every unstaged/untracked changes with the one upstream
	//    3.3. Save the patch and compress it
	// 4. Submit build!

	ancestorRef, commitCount, branch, err := fastFindAncestor(ctx, workRepo)
	if err != nil { // Error
		return err
	}
	p.branch = branch
	p.baseCommit = ancestorRef.String()

	head, err := workRepo.Head()
	if err != nil {
		return fmt.Errorf("couldn't find HEAD commit: %w", err)
	}
	headCommit, err := workRepo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("couldn't find HEAD commit: %w", err)
	}
	ancestorCommit, err := workRepo.CommitObject(ancestorRef)
	if err != nil {
		return fmt.Errorf("couldn't find merge-base commit: %w", err)
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
			return fmt.Errorf("patch generation failed: %w", err)
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
			return fmt.Errorf("couldn't get current worktree: %w", err)
		}

		log.Infof(ctx, "Generating patch for local changes...")

		// Save files before committing.
		log.Debugf(ctx, "Start backing up the worktree-save")
		saver, err := newWorktreeSave(targetPackage.Path, ggit.Hash(headCommit.Hash), p.backupWorktree)
		if err != nil {
			return err
		}
		if err := p.traverseChanges(ctx, g, saver); err != nil {
			return err
		}
		resetDone := false
		if err := saver.save(ctx); err != nil {
			return err
		}
		defer func() {
			if !resetDone {
				log.Debugf(ctx, "Reset failed, restoring...")
				if err := saver.restore(ctx); err != nil {
					log.Errorf(ctx,
						"Unable to restore kept files at %s: %v\n"+
							"     Please consider unarchiving that package",
						saver.saveFilePath(),
						err)
				}
			}
		}()

		log.Debugf(ctx, "Committing temporary changes")
		latest, err := commitTempChanges(worktree, headCommit)
		if err != nil {
			return fmt.Errorf("commit to temporary cloned repository failed: %w", err)
		}

		tempCommit, err := workRepo.CommitObject(latest)
		if err != nil {
			return fmt.Errorf("can't find commit %q: %w", latest, err)
		}

		log.Debugf(ctx, "Starting the actual patch generation...")
		patch, err := ancestorCommit.Patch(tempCommit)
		if err != nil {
			return fmt.Errorf("patch generation failed: %w", err)
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

	if p.dryRun {
		log.Infof(ctx, "Dry run ended, build not submitted")
		return nil
	}

	if err := p.submitBuild(ctx, project, target.Tags); err != nil {
		return fmt.Errorf("unable to submit build: %w", err)
	}
	return nil
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

func (p *remoteCmd) traverseChanges(ctx context.Context, g *ggit.Git, saver *worktreeSave) error {
	workTree, err := g.WorkTree(ctx)
	if err != nil {
		return fmt.Errorf("traverse changes: %w", err)
	}
	status, err := g.Status(ctx, ggit.StatusOptions{
		DisableRenames: true,
	})
	if err != nil {
		return fmt.Errorf("traverse changes: %w", err)
	}

	var addList []ggit.Pathspec
	for _, ent := range status {
		if ent.Code[1] == ' ' {
			// If file is already staged, then skip.
			continue
		}
		var err error
		addList, err = findFilesToAdd(ctx, g, workTree, addList, ent.Name)
		if err != nil {
			return fmt.Errorf("traverse changes: %w", err)
		}

		if !ent.Code.IsMissing() { // No need to add deletion to the saver, right?
			if err = saver.add(ctx, filepath.FromSlash(string(ent.Name))); err != nil {
				return fmt.Errorf("traverse changes: %w", err)
			}
		}
	}

	err = g.Add(ctx, addList, ggit.AddOptions{
		IncludeIgnored: true,
	})
	if err != nil {
		return fmt.Errorf("traverse changes: %w", err)
	}
	return nil
}

// findFilesToAdd finds files to stage in Git, recursing into directories and
// ignoring any non-text files.
func findFilesToAdd(ctx context.Context, g *ggit.Git, workTree string, dst []ggit.Pathspec, file ggit.TopPath) ([]ggit.Pathspec, error) {
	realPath := filepath.Join(workTree, filepath.FromSlash(string(file)))
	fi, err := os.Stat(realPath)
	if os.IsNotExist(err) {
		return dst, nil
	}
	if err != nil {
		return dst, fmt.Errorf("find files to git add: %w", err)
	}

	if !fi.IsDir() {
		binary, err := isBinary(realPath)
		if err != nil {
			return dst, fmt.Errorf("find files to git add: %w", err)
		}
		log.Debugf(ctx, "%s is binary = %t", file, binary)
		if binary {
			log.Infof(ctx, "Skipping binary file %s", realPath)
			return dst, nil
		}
		return append(dst, file.Pathspec()), nil
	}

	log.Debugf(ctx, "Added a dir, checking its contents: %s", file)
	dir, err := ioutil.ReadDir(realPath)
	if err != nil {
		return dst, fmt.Errorf("find files to git add: %w", err)
	}
	for _, f := range dir {
		var err error
		dst, err = findFilesToAdd(ctx, g, workTree, dst, ggit.TopPath(path.Join(string(file), f.Name())))
		if err != nil {
			return dst, err
		}
	}
	return dst, nil
}

// isBinary returns whether a file contains a NUL byte near the beginning of the file.
func isBinary(filePath string) (bool, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer r.Close()

	buf := make([]byte, 8000)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		// Ignore EOF, since it's fine for the file to be shorter than the buffer size.
		// Otherwise, wrap the error. We don't fully stop the control flow here because
		// we may still have read enough data to make a determination.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			err = nil
		} else {
			err = fmt.Errorf("check for binary: %w", err)
		}
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true, err
		}
	}
	return false, err
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
	req := &http.Request{
		Method: http.MethodPost,
		URL:    apiURL,
		Header: http.Header{
			http.CanonicalHeaderKey("YB_API_TOKEN"): {userToken},
			headers.ContentType:                     {"application/x-www-form-urlencoded"},
		},
		GetBody: func() (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader(formData.Encode())), nil
		},
	}
	req.Body, _ = req.GetBody()
	res, err := http.DefaultClient.Do(req)
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

// An apiProject is a YourBase project as returned by the API.
type apiProject struct {
	ID          int    `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	OrgSlug     string `json:"organization_slug"`
}

func (p *remoteCmd) fetchProject(ctx context.Context, urls []string) (*apiProject, error) {
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
	project := new(apiProject)
	err = json.Unmarshal(body, project)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (cmd *remoteCmd) savePatch() error {

	err := ioutil.WriteFile(cmd.patchPath, cmd.patchData, 0644)

	if err != nil {
		return fmt.Errorf("Couldn't save a local patch file at: %s, because: %v", cmd.patchPath, err)
	}

	return nil
}

func (cmd *remoteCmd) submitBuild(ctx context.Context, project *apiProject, tagMap map[string]string) error {

	startTime := time.Now()

	userToken, err := ybconfig.UserToken()
	if err != nil {
		return err
	}

	patchBuffer := new(bytes.Buffer)
	xzWriter, err := xz.NewWriter(patchBuffer)
	if err != nil {
		return fmt.Errorf("submit build: compress patch: %w", err)
	}
	if _, err := xzWriter.Write(cmd.patchData); err != nil {
		return fmt.Errorf("submit build: compress patch: %w", err)
	}
	if err := xzWriter.Close(); err != nil {
		return fmt.Errorf("submit build: compress patch: %w", err)
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
	var uiURL *url.URL
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
				if uiURL != nil {
					log.Infof(ctx, "Build Log: %v", uiURL)
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
				if uiURL != nil {
					log.Infof(ctx, "Build Log: %v", uiURL)
				}
			}
			if !buildSuccess {
				buildSuccess = strings.Count(string(msg), "-- BUILD SUCCEEDED --") > 0
			}
			os.Stdout.Write(msg)
		}
	}
}

type worktreeSave struct {
	path  string
	hash  ggit.Hash
	files []string
}

func newWorktreeSave(path string, hash ggit.Hash, enabled bool) (*worktreeSave, error) {
	if !enabled {
		return nil, nil
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("save worktree state: %w", err)
	}
	return &worktreeSave{
		path: path,
		hash: hash,
	}, nil
}

func (w *worktreeSave) hasFiles() bool {
	return w != nil && len(w.files) > 0
}

func (w *worktreeSave) add(ctx context.Context, file string) error {
	if w == nil {
		return nil
	}
	fullPath := filepath.Join(w.path, file)
	if _, err := os.Lstat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("save worktree state: %w", err)
	}
	log.Debugf(ctx, "Saving %s to the tarball", file)
	w.files = append(w.files, file)
	return nil
}

func (w *worktreeSave) saveFilePath() string {
	return filepath.Join(w.path, fmt.Sprintf(".yb-worktreesave-%v.tar", w.hash))
}

func (w *worktreeSave) save(ctx context.Context) error {
	if !w.hasFiles() {
		return nil
	}
	log.Debugf(ctx, "Saving a tarball with all the worktree changes made")
	tar := archiver.Tar{
		MkdirAll: true,
	}
	if err := tar.Archive(w.files, w.saveFilePath()); err != nil {
		return fmt.Errorf("save worktree state: %w", err)
	}
	return nil
}

func (w *worktreeSave) restore(ctx context.Context) error {
	if !w.hasFiles() {
		return nil
	}
	log.Debugf(ctx, "Restoring the worktree tarball")
	pkgFile := w.saveFilePath()
	if _, err := os.Lstat(pkgFile); os.IsNotExist(err) {
		return fmt.Errorf("restore worktree state: %w", err)
	}
	tar := archiver.Tar{OverwriteExisting: true}
	if err := tar.Unarchive(pkgFile, w.path); err != nil {
		return fmt.Errorf("restore worktree state: %w", err)
	}
	if err := os.Remove(pkgFile); err != nil {
		log.Warnf(ctx, "Failed to clean up temporary worktree save: %v", err)
	}
	return nil
}
