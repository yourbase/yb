package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type remoteCmd struct {
	capitalize bool
}

func (*remoteCmd) Name() string     { return "remotebuild" }
func (*remoteCmd) Synopsis() string { return "Build remotely." }
func (*remoteCmd) Usage() string {
	return "Build remotely"
}

func (p *remoteCmd) SetFlags(f *flag.FlagSet) {
}

func (p *remoteCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	workspace := LoadWorkspace()
	targetPackage := workspace.Target

	fmt.Printf("Remotely building target package %s...\n", targetPackage)
	packagePath := workspace.PackagePath(targetPackage)
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

	project, err := fetchProject(repoUrls)

	if err != nil {
		fmt.Printf("Error fetching project metadata: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Printf("Project key: %d\n", project.Id)

	err = submitBuild(project)

	if err != nil {
		fmt.Printf("Unable to submit build: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func ApiUrl(path string) string {
	apiBaseURL, exists := os.LookupEnv("YOURBASE_API_URL")
	if !exists {
		apiBaseURL = "https://yb-api.herokuapp.com"
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
	req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/json")
	res, _ := client.Do(req)
	return res, nil

}
func postToApi(path string, formData url.Values) (*http.Response, error) {
	userToken, err := getUserToken()

	if err != nil {
		return nil, err
	}

	apiURL := ApiUrl(path)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, _ := client.Do(req)
	return res, nil
}

func postToDispatcher(path string, formData url.Values) (*http.Response, error) {
	userToken, err := getUserToken()

	if err != nil {
		return nil, err
	}

	dispatcherBaseURL, exists := os.LookupEnv("DISPATCHER_URL")
	if !exists {
		dispatcherBaseURL = "https://yb-dispatcher.herokuapp.com"
	}

	dispatcherURL := fmt.Sprintf("%s/%s", dispatcherBaseURL, path)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", dispatcherURL, strings.NewReader(formData.Encode()))
	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, _ := client.Do(req)

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
		log.Fatalln(err)
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
		apiBaseURL = "https://yb-api.herokuapp.com"
	}

	apiURL := fmt.Sprintf("%s/users/whoami", apiBaseURL)

	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("YB_API_TOKEN", userToken)
	res, _ := client.Do(req)

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

func submitBuild(project *Project) error {

	userToken, err := getUserToken()

	if err != nil {
		return err
	}

	formData := url.Values{
		"project_id":    {strconv.Itoa(project.Id)},
		"repository_id": {project.Repository},
		"api_key":       {userToken},
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
	}

	return nil

}
