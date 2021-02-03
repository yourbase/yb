package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourbase/commons/ini"
	"github.com/yourbase/yb/internal/config"
	"go4.org/xdgdir"
	"zombiezen.com/go/log"
)

var (
	VARS = []string{"environment", "log-level", "log-section", "no-pretty-output"}
)

func newConfigCmd(cfg ini.FileSet) *cobra.Command {
	group := &cobra.Command{
		Use:           "config",
		Short:         "Print or change settings",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	group.AddCommand(newConfigGetCmd(cfg), newConfigSetCmd(cfg))
	return group
}

func newConfigGetCmd(cfg ini.FileSet) *cobra.Command {
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
					fmt.Println(config.Get(cfg, "defaults", k))
					return nil
				}
			}
			return fmt.Errorf("unknown config variable %q (must be one of %s)", k, strings.Join(VARS, ", "))
		},
	}
}

func newConfigSetCmd(cfg ini.FileSet) *cobra.Command {
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
					if len(cfg) == 0 {
						return fmt.Errorf("%v not set", xdgdir.Config)
					}
					cfg.Set("defaults", k, v)
					if err := config.Save(cfg[0]); err != nil {
						return err
					}
					log.Infof(ctx, "Configuration done")
					return nil
				}
			}
			return fmt.Errorf("unknown config variable %q (must be one of %s)", k, strings.Join(VARS, ", "))
		},
	}
}
