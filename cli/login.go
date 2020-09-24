package cli

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
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/types"
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
	tokenUrl, err := ybconfig.UserSettingsUrl()
	if err != nil {
		log.Errorf("Couldn't determine login URL: %v\n", err)
		return subcommands.ExitFailure
	}

	tokenPrompt := fmt.Sprintf("Open up %s and then paste the token here.", tokenUrl)
	fmt.Println(tokenPrompt)
	fmt.Print("API Token: ")
	apiToken, _ := reader.ReadString('\n')
	apiToken = strings.TrimSuffix(apiToken, "\n")

	validationUrl, err := ybconfig.TokenValidationUrl(apiToken)

	if err != nil {
		log.Errorf("Unable to get token validation URL: %v\n", err)
		return subcommands.ExitFailure
	}

	resp, err := http.Get(validationUrl)

	if err != nil {
		log.Errorf("Couldn't make validation request: %v\n", err)
		return subcommands.ExitFailure
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 401 {
		log.Errorf("Invalid token provided, please check it\n")
		return subcommands.ExitFailure
	}

	if resp.StatusCode != 200 {
		log.Errorf("Oops: HTTP Status %d that's us, not you, please try again later", resp.StatusCode)
		return subcommands.ExitFailure
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Couldn't parse response body: %s\n", err)
		return subcommands.ExitFailure
	}
	var tokenResponse types.TokenResponse
	err = json.Unmarshal(body, &tokenResponse)

	if err != nil {
		log.Errorf("Couldn't parse response body: %s\n", err)
		return subcommands.ExitFailure
	}

	if !tokenResponse.TokenOK {
		log.Errorf("Token provided is invalid, please check it\n")
		return subcommands.ExitFailure
	}

	if err = ybconfig.SetConfigValue("user", "api_key", apiToken); err != nil {
		log.Errorf("Cannot store API token: %v\n", err)
		return subcommands.ExitFailure
	}

	log.Infoln("API token saved to the config file")
	return subcommands.ExitSuccess
}
