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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"

	ybconfig "github.com/yourbase/yb/config"
)

type RemoteCmd struct {
	target         string
	baseCommit     string
	commonCommit   string
	branch         string
	noAcceleration bool
	patchFile      string
	patchPath      string
	noDeleteTemp   bool
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
	f.BoolVar(&p.noDeleteTemp, "no-delete-temp", false, "Delete temp generated data")

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
	packagePath := targetPackage.Path
	workRepo, err := git.PlainOpen(packagePath)

	if err != nil {
		fmt.Printf("Error opening repository %s: %v\n", packagePath, err)
		return subcommands.ExitFailure
	}

	list, err := workRepo.Remotes()

	if err != nil {
		fmt.Printf("Error getting remotes for %s: %v\n", packagePath, err)
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

	// Invoke Patch to get one patch file to send to the API
	if project.Repository == "" {
		projectUrl, err := ybconfig.ManagementUrl(fmt.Sprintf("%s/%s", project.OrgSlug, project.Label))
		if err != nil {
			fmt.Printf("Unable to generate project URL: %v\n", err)
			return subcommands.ExitFailure
		}

		fmt.Printf("Empty repository for project %s, please check your project settings at %s", project.Label, projectUrl)
		return subcommands.ExitFailure
	}

	patchFile := ""
	if file, err := ioutil.TempFile("", "yb-*.patch"); err != nil {
		fmt.Printf("Couldn't create temp file: %s, error: %s", patchFile, err)
		return subcommands.ExitFailure
	} else {
		patchFile = file.Name()

		if err = file.Close(); err != nil {
			//Ignore
		}
		// We just need the unique name
		os.Remove(patchFile)
	}

	cloneDir, err := ioutil.TempDir("", "yb-clone-")
	if err != nil {
		fmt.Printf("Unable to create clone dir to fetch and checkout remote repository '%v': %v\n", project.Repository, err)
		return subcommands.ExitFailure
	}
	if !p.noDeleteTemp {
		defer os.RemoveAll(cloneDir)
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

	LOGGER.Debugf("Cloning repo %s into %s, using branch '%s'", project.Repository, cloneDir, foundBranch)

	clonedRepo, err := CloneRepository(project.Repository, cloneDir, foundBranch)
	if err != nil {
		fmt.Printf("Unable to clone %v, using branch '%v': %v\n", project.Repository, foundBranch, err)

		clonedRepo, err = CloneRepository(project.Repository, cloneDir, "master")
		if err != nil {
			fmt.Printf("Unable to clone %v using the master branch: %v\n", project.Repository, err)
			return subcommands.ExitFailure
		}
		p.branch = "master"
	} else {
		p.branch = foundBranch
	}

	foundCommit, err := defineCommit(workRepo, p.baseCommit)
	if err != nil {
		fmt.Printf("Error finding and deciding which base-commit to use: %v\n", err)
		return subcommands.ExitFailure
	} else {
		fmt.Printf("Found common commit: '%v'\n", foundCommit)
		p.baseCommit = foundCommit
	}

	targetSet := commitSet(workRepo)
	if targetSet == nil {
		fmt.Printf("Branch isn't on remote yet and couldn't build a commit set for comparing\n")
		return subcommands.ExitFailure
	}
	p.commonCommit = findCommonAncestor(clonedRepo, targetSet).String()

	p.commonCommit = findCommonAncestor(clonedRepo, targetSet).String()

	patchCmd := &PatchCmd{
		targetRepository: cloneDir,
		patchFile:        patchFile,
	}

	LOGGER.Debugf("targetRepository: %v patchFile: %v\n", patchCmd.targetRepository, patchCmd.patchFile)

	if patchCmd.Execute(context.Background(), f, nil) != subcommands.ExitSuccess {
		fmt.Printf("Patch command failed\n")
		return subcommands.ExitFailure
	}

	p.patchFile = patchFile

	if p.patchPath != "" {
		if err := savePatch(p); err != nil {
			fmt.Printf("\nUnable to save copy of generated patch: %v\n\n", err)
		} else {
			fmt.Printf("Patch copy saved at: %v\n", p.patchPath)
		}
	}

	err = submitBuild(project, p, target.Tags)

	if err != nil {
		fmt.Printf("Unable to submit build: %v\n", err)
		return subcommands.ExitFailure
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
		return nil, fmt.Errorf("Unable to generate API URL: %v")
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
			return "", fmt.Errorf("No Head: %v\n", err)
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

	data, _ := ioutil.ReadFile(cmd.patchFile)
	err := ioutil.WriteFile(cmd.patchPath, data, 0644)

	if err != nil {
		return fmt.Errorf("Couldn't save a local patch file at: %v, because: %v", err)
	}

	return nil
}

func submitBuild(project *Project, cmd *RemoteCmd, tagMap map[string]string) error {

	userToken, err := ybconfig.UserToken()

	if err != nil {
		return err
	}

	patchData, _ := ioutil.ReadFile(cmd.patchFile)

	patchBuffer := bytes.NewBuffer(patchData)

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
		"commit":     {cmd.commonCommit},
		"branch":     {cmd.branch},
	}

	tags := make([]string, 0)
	for k, v := range tagMap {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	for _, tag := range tags {
		formData.Add("tags[]", tag)
	}

	LOGGER.Debugf("Common commit: %s\n", cmd.commonCommit)
	cliUrl, err := ybconfig.ApiUrl("builds/cli")
	if err != nil {
		LOGGER.Debugf("Unable to generate CLI URL: %v\n", err)
	}
	LOGGER.Debugf("Calling backend (%s) with the following values: %v\n", cliUrl, formData)

	if !cmd.noDeleteTemp {
		defer os.Remove(cmd.patchFile)
	}

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
