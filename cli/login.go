package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"syscall"

	. "github.com/yourbase/yb/plumbing"
	. "github.com/yourbase/yb/types"
)

type LoginCmd struct {
}

func (*LoginCmd) Name() string     { return "login" }
func (*LoginCmd) Synopsis() string { return "Log into YB" }
func (*LoginCmd) Usage() string {
	return `login`
}

func (p *LoginCmd) SetFlags(f *flag.FlagSet) {
}

func (p *LoginCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSuffix(email, "\n")

	fmt.Print("Password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	password := string(bytePassword)
	fmt.Println()

	values := map[string]string{"email": email, "password": password}
	jsonData, _ := json.Marshal(values)
	loginUrl := ApiUrl("/users/login")

	resp, err := http.Post(loginUrl, "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		fmt.Printf("Couldn't make authentication request: %v\n", err)
		return subcommands.ExitFailure
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 401 {
		fmt.Printf("Invalid credentials, please check email/password\n")
		return subcommands.ExitFailure
	}

	if resp.StatusCode != 200 {
		fmt.Printf("Other problems going on: HTTP Status %d\n", resp.StatusCode)
		return subcommands.ExitFailure
	}

	body, err := ioutil.ReadAll(resp.Body)
	var loginResponse LoginResponse
	err = json.Unmarshal(body, &loginResponse)

	if err != nil {
		fmt.Printf("Couldn't parse response body: %s\n", err)
		return subcommands.ExitFailure
	}

	apiToken := loginResponse.ApiToken

	if err = SetConfigValue("user", "api_key", apiToken); err != nil {
		fmt.Printf("Cannot store API token: %v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
