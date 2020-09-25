package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/config"
	"github.com/yourbase/yb/plumbing/log"
)

// TokenCmd represents an invocation of `yb token`, which outputs the saved
// token from `yb login` to stdout. This can be used
type TokenCmd struct {
	Version string
	Channel string
}

// Name returns the literal text of the token command.
func (*TokenCmd) Name() string { return "token" }

// Synopsis returns the shortform description of the token command.
func (*TokenCmd) Synopsis() string {
	return "Print an auth token"
}

// Usage returns usage information for thet token command.
func (*TokenCmd) Usage() string {
	return `token
Prints a YourBase auth token to stdout. Compose this with other tools to make interacting with the YourBase API easier.
`
}

// SetFlags describes the flags available to the token command.
func (p *TokenCmd) SetFlags(f *flag.FlagSet) {
}

// Execute runs the token command.
func (p *TokenCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	token, err := config.GetConfigValue("user", "api_key")
	if err != nil {
		log.Errorf("Cannot get auth token: %v", err)
		return subcommands.ExitFailure
	}

	fmt.Println(token)
	return subcommands.ExitSuccess
}
