package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/johnewart/subcommands"
	ybconfig "github.com/yourbase/yb/config"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

type LoginCmd struct {
}

func (*LoginCmd) Name() string     { return "login" }
func (*LoginCmd) Synopsis() string { return "Log into YB" }
func (*LoginCmd) Usage() string {
	return `Usage: login
Configure yb to act as you in the YourBase API.
`
}

func (p *LoginCmd) SetFlags(f *flag.FlagSet) {
}

func (p *LoginCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	reader := bufio.NewReader(os.Stdin)
	tokenURL, err := ybconfig.UserSettingsURL()
	if err != nil {
		log.Errorf(ctx, "Couldn't determine login URL: %v\n", err)
		return subcommands.ExitFailure
	}

	tokenPrompt := fmt.Sprintf("Open up %s and then paste the token here.", tokenURL)
	fmt.Println(tokenPrompt)
	fmt.Print("API Token: ")
	apiToken, _ := reader.ReadString('\n')
	apiToken = strings.TrimSuffix(apiToken, "\n")

	validationURL, err := ybconfig.TokenValidationURL(apiToken)

	if err != nil {
		log.Errorf(ctx, "Unable to get token validation URL: %v\n", err)
		return subcommands.ExitFailure
	}

	resp, err := http.Get(validationURL)

	if err != nil {
		log.Errorf(ctx, "Couldn't make validation request: %v\n", err)
		return subcommands.ExitFailure
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 401 {
		log.Errorf(ctx, "Invalid token provided, please check it\n")
		return subcommands.ExitFailure
	}

	if resp.StatusCode != 200 {
		log.Errorf(ctx, "Oops: HTTP Status %d that's us, not you, please try again later", resp.StatusCode)
		return subcommands.ExitFailure
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf(ctx, "Couldn't parse response body: %s\n", err)
		return subcommands.ExitFailure
	}
	var tokenResponse types.TokenResponse
	err = json.Unmarshal(body, &tokenResponse)

	if err != nil {
		log.Errorf(ctx, "Couldn't parse response body: %s\n", err)
		return subcommands.ExitFailure
	}

	if !tokenResponse.TokenOK {
		log.Errorf(ctx, "Token provided is invalid, please check it\n")
		return subcommands.ExitFailure
	}

	if err = ybconfig.Set("user", "api_key", apiToken); err != nil {
		log.Errorf(ctx, "Cannot store API token: %v\n", err)
		return subcommands.ExitFailure
	}

	log.Infof(ctx, "API token saved to the config file")
	return subcommands.ExitSuccess
}
