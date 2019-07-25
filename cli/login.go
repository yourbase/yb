package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/johnewart/subcommands"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	. "github.com/yourbase/yb/types"

	ybconfig "github.com/yourbase/yb/config"
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
	fmt.Println(fmt.Sprintf("Open up %s and then paste the token here.", ybconfig.ManagementUrl("user/apitoken")))
	fmt.Println()

	fmt.Print("Token: ")
	apiToken, _ := reader.ReadString('\n')
	apiToken = strings.TrimSuffix(apiToken, "\n")

	fmt.Println()

	resp, err := http.Get(ybconfig.ApiUrl(fmt.Sprintf("/apikey/validate/%s", apiToken)))

	if err != nil {
		fmt.Printf("Couldn't make validation request: %v\n", err)
		return subcommands.ExitFailure
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 401 {
		fmt.Printf("Invalid token provided, please check it\n")
		return subcommands.ExitFailure
	}

	if resp.StatusCode != 200 {
		fmt.Printf("Oops: HTTP Status %d that's us, not you, please try again later\n", resp.StatusCode)
		return subcommands.ExitFailure
	}

	body, err := ioutil.ReadAll(resp.Body)
	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)

	if err != nil {
		fmt.Printf("Couldn't parse response body: %s\n", err)
		return subcommands.ExitFailure
	}

	if !tokenResponse.TokenOK {
		fmt.Printf("Token provided is invalid, please check it\n")
		return subcommands.ExitFailure
	}

	if err = ybconfig.SetConfigValue("user", "api_key", apiToken); err != nil {
		fmt.Printf("Cannot store API token: %v\n", err)
		return subcommands.ExitFailure
	} else {
		fmt.Println("API token saved to the config file")
	}

	return subcommands.ExitSuccess
}
