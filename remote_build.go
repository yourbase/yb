package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"
	"gopkg.in/src-d/go-git.v4"
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

	project := fetchProject(repoUrls)
	fmt.Printf("Project key: %d\n", project.Id)

	submitBuild(project)

	return subcommands.ExitSuccess
}

func postToApi(path string, formData url.Values) (*http.Response, error) {
	apiBaseURL, exists := os.LookupEnv("YOURBASE_API_URL")
	if !exists {
		apiBaseURL = "https://yb-api.herokuapp.com"
	}

	apiURL := fmt.Sprintf("%s/%s", apiBaseURL, path)

	return http.PostForm(apiURL, formData)
}

func postToDispatcher(path string, formData url.Values) (*http.Response, error) {
	dispatcherBaseURL, exists := os.LookupEnv("DISPATCHER_URL")
	if !exists {
		dispatcherBaseURL = "https://yb-dispatcher.herokuapp.com"
	}

	dispatcherURL := fmt.Sprintf("%s/%s", dispatcherBaseURL, path)

	return http.PostForm(dispatcherURL, formData)
}

func fetchProject(urls []string) *Project {
	v := url.Values{}
	for _, u := range urls {
		fmt.Printf("Adding remote URL %s to search...\n", u)
		v.Add("urls[]", u)
	}
	resp, err := postToApi("search/projects", v)

	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var project Project
	err = json.Unmarshal(body, &project)
	if err != nil {
		fmt.Printf("Couldn't parse response body: %s\n", err)
		return nil
	}

	return &project
}

func submitBuild(project *Project) {

	formData := url.Values{
		"project_id":    {strconv.Itoa(project.Id)},
		"repository_id": {project.Repository},
		"api_key":       {"12345"},
	}

	resp, err := postToDispatcher("builds", formData)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//fmt.Println(resp.Body)
	if err != nil {
		fmt.Printf("Couldn't read response body: %s\n", err)
	}

	response := string(body)

	fmt.Println(response)
	if strings.HasPrefix(response, "ws:") || strings.HasPrefix(response, "wss:") {
		fmt.Println("Build output:")
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), response)
		if err != nil {
			fmt.Printf("can not connect: %v\n", err)
		} else {
			for {
				msg, _, err := wsutil.ReadServerData(conn)
				if err != nil {
					fmt.Printf("can not receive: %v\n", err)
					return
				} else {
					//fmt.Printf("receive: %s，type: %v\n", msg, op)
					fmt.Printf("%s", msg)
				}
				//time.Sleep(time.Duration(1) * time.Second)
			}

			err = conn.Close()
			if err != nil {
				fmt.Printf("can not close: %v\n", err)
			} else {
				fmt.Printf("closed\n")
			}
		}
	}
}
