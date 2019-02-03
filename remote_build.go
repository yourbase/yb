package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/johnewart/subcommands"
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
	repo := fmt.Sprintf("https://github.com/johnewart/%s.git", targetPackage)
	submitBuild(repo)

	return subcommands.ExitSuccess
}

func submitBuild(repository string) {
	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), "ws://127.0.0.1:8080/")
	if err != nil {
		fmt.Printf("can not connect: %v\n", err)
	} else {
		fmt.Printf("connected\n")
		cmd := fmt.Sprintf("BUILD %s", repository)
		msg := []byte(cmd)
		err = wsutil.WriteClientMessage(conn, ws.OpText, msg)
		if err != nil {
			fmt.Printf("can not send: %v\n", err)
			return
		} else {
			fmt.Printf("send: %s, type: %v\n", msg, ws.OpText)
		}
		for {
			msg, op, err := wsutil.ReadServerData(conn)
			if err != nil {
				fmt.Printf("can not receive: %v\n", err)
				return
			} else {
				fmt.Printf("receive: %s，type: %v\n", msg, op)
			}
			time.Sleep(time.Duration(1) * time.Second)
		}

		err = conn.Close()
		if err != nil {
			fmt.Printf("can not close: %v\n", err)
		} else {
			fmt.Printf("closed\n")
		}
	}
}
