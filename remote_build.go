package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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
	//repo := fmt.Sprintf("https://github.com/johnewart/%s.git", targetPackage)
	repo := fmt.Sprintf("johnewart/%s", targetPackage)
	submitBuild(repo)

	return subcommands.ExitSuccess
}

func submitBuild(repository string) {

	formData := url.Values{
		"repository":   {repository},
		"organization": {"johnewart"},
	}

	resp, err := http.PostForm("http://127.0.0.1:8080/builds", formData)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//fmt.Println(resp.Body)
	if err != nil {
		fmt.Printf("Couldn't read respons body: %s\n", err)
	}

	response := string(body)

	fmt.Println(response)
	if strings.HasPrefix(response, "ws:") {
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
