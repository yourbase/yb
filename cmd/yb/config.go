package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb/internal/config"
	"zombiezen.com/go/log"
)

var (
	VARS = []string{"environment", "log-level", "log-section", "no-pretty-output"}
)

func newConfigCmd() *cobra.Command {
	group := &cobra.Command{
		Use:           "config",
		Short:         "Print or change settings",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	group.AddCommand(newConfigGetCmd(), newConfigSetCmd())
	return group
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "get KEY",
		Short:                 "Print configuration setting",
		SilenceErrors:         true,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k := args[0]
			for _, configVar := range VARS {
				if k == configVar {
					v, err := config.Get("defaults", k)
					if err != nil {
						return err
					}
					fmt.Println(v)
					return nil
				}
			}
			return fmt.Errorf("unknown config variable %q (must be one of %s)", k, strings.Join(VARS, ", "))
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "set KEY=VALUE",
		Short:                 "Change configuration setting",
		SilenceErrors:         true,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg, received %d", len(args))
			}
			if !strings.Contains(args[0], "=") {
				return fmt.Errorf("argument must be in the form KEY=VALUE")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			i := strings.Index(args[0], "=")
			k, v := args[0][:i], args[0][i+1:]
			for _, configVar := range VARS {
				if k == configVar {
					config.Set("defaults", k, v)
					log.Infof(ctx, "Configuration done")
					return nil
				}
			}
			return fmt.Errorf("unknown config variable %q (must be one of %s)", k, strings.Join(VARS, ", "))
		},
	}
}
