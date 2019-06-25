package cli

import (
	"bytes"
	"context"
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

	. "github.com/yourbase/yb/packages"
	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
	. "github.com/yourbase/yb/workspace"
)

type RemoteCmd struct {
	capitalize bool
}

func (*RemoteCmd) Name() string     { return "remotebuild" }
func (*RemoteCmd) Synopsis() string { return "Build remotely." }
func (*RemoteCmd) Usage() string {
	return "Build remotely"
}

func (p *RemoteCmd) SetFlags(f *flag.FlagSet) {
}

func (p *RemoteCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	buildTarget := "default"

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
		if _, err := manifest.BuildTarget(buildTarget); err != nil {
			fmt.Printf("Build target %s specified but it doesn't exist!\n", buildTarget)
			fmt.Printf("Valid build targets: %s\n", strings.Join(manifest.BuildTargetList(), ", "))
		}
	}

	fmt.Printf("Remotely building '%s' ...\n", buildTarget)
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
		fmt.Println(r)
		c := r.Config()
		for _, u := range c.URLs {
			repoUrls = append(repoUrls, u)
		}
	}

	headRef, err := workRepo.Head()

	branch := fmt.Sprintf("%s", headRef.Name())

	if strings.Contains(branch, "refs/heads") {
		branch = strings.Replace(branch, "refs/heads/", "", -1)
	}

	fmt.Printf("Remote building branch: %s\n", branch)

	commitHash := fmt.Sprintf("%s", headRef.Hash())

	fmt.Printf("Remote building commit: %s\n", commitHash)
	project, err := fetchProject(repoUrls)

	if err != nil {
		fmt.Printf("Error fetching project metadata: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Printf("Project key: %d\n", project.Id)

	err = submitBuild(project, target.Tags, commitHash, branch)

	if err != nil {
		fmt.Printf("Unable to submit build: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func ManagementUrl(path string) string {
	managementBaseURL, exists := os.LookupEnv("YOURBASE_UI_URL")
	if !exists {
		managementBaseURL = "https://app.yourbase.io"
	}

	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	managementURL := fmt.Sprintf("%s%s", managementBaseURL, path)

	return managementURL
}

func ApiUrl(path string) string {
	apiBaseURL, exists := os.LookupEnv("YOURBASE_API_URL")
	if !exists {
		apiBaseURL = "https://api.yourbase.io"
	}

	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	apiURL := fmt.Sprintf("%s%s", apiBaseURL, path)

	return apiURL
}

func postJsonToApi(path string, jsonData []byte) (*http.Response, error) {
	userToken, err := getUserToken()

	if err != nil {
		return nil, err
	}

	apiURL := ApiUrl(path)

	client := &http.Client{}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	return res, err

}
func postToApi(path string, formData url.Values) (*http.Response, error) {
	userToken, err := getUserToken()

	if err != nil {
		return nil, err
	}

	apiURL := ApiUrl(path)
	client := &http.Client{}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func postToDispatcher(path string, formData url.Values) (*http.Response, error) {
	userToken, err := getUserToken()

	if err != nil {
		return nil, err
	}

	dispatcherBaseURL, exists := os.LookupEnv("DISPATCHER_URL")
	if !exists {
		dispatcherBaseURL = "https://router.yourbase.io"
	}

	dispatcherURL := fmt.Sprintf("%s/%s", dispatcherBaseURL, path)

	client := &http.Client{}
	req, err := http.NewRequest("POST", dispatcherURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func getUserToken() (string, error) {
	token, exists := os.LookupEnv("YB_USER_TOKEN")
	if !exists {
		if token, err := GetConfigValue("user", "api_key"); err != nil {
			return "", fmt.Errorf("Unable to find YB token in config file or environment.")
		} else {
			return token, nil
		}
	} else {
		return token, nil
	}
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

func fetchUserEmail() (string, error) {
	userToken, err := getUserToken()
	if err != nil {
		return "", err
	}

	apiBaseURL, exists := os.LookupEnv("YOURBASE_API_URL")
	if !exists {
		apiBaseURL = "https://api.yourbase.io"
	}

	apiURL := fmt.Sprintf("%s/users/whoami", apiBaseURL)

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("YB_API_TOKEN", userToken)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode == 200 {
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)

		if err != nil {
			return "", err
		}

		email := string(body)
		return email, nil
	} else {
		return "", fmt.Errorf("User could not be found using your API token, please double-check and try again")
	}
}

func submitBuild(project *Project, tagMap map[string]string, commit string, branch string) error {

	userToken, err := getUserToken()

	if err != nil {
		return err
	}

	target := "default"

	formData := url.Values{
		"project_id":    {strconv.Itoa(project.Id)},
		"repository_id": {project.Repository},
		"api_key":       {userToken},
		"target":        {target},
		"commit":        {commit},
		"branch":        {branch},
	}

	tags := make([]string, 0)
	for k, v := range tagMap {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	for _, tag := range tags {
		formData.Add("tags[]", tag)
	}

	resp, err := postToDispatcher("builds", formData)
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

	if strings.HasPrefix(response, "ws:") || strings.HasPrefix(response, "wss:") {
		fmt.Printf("Streaming build output from %s\n", response)
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), response)
		if err != nil {
			return fmt.Errorf("can not connect: %v\n", err)
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
				fmt.Printf("can not close: %v\n", err)
			} else {
				fmt.Printf("closed\n")
			}
		}
	} else {
		fmt.Println(response)
	}

	return nil

}
