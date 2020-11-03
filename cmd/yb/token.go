package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb/internal/config"
)

// tokenCmd represents an invocation of `yb token`, which outputs the saved
// token from `yb login` to stdout. This can be used
type tokenCmd struct{}

func newTokenCmd() *cobra.Command {
	p := new(tokenCmd)
	return &cobra.Command{
		Use:           "token",
		Short:         "Print an auth token",
		Long:          `Print a YourBase auth token to stdout. Compose this with other tools to interact with the YourBase API more easily.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return p.run(cmd.Context())
		},
	}
}

func (*tokenCmd) run(ctx context.Context) error {
	token, err := config.Get("user", "api_key")
	if err != nil {
		return fmt.Errorf("get auth token: %w", err)
	}
	_, err = fmt.Println(token)
	return err
}
