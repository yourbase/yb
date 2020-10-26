package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/yourbase/yb/types"
	"zombiezen.com/go/log"
)

type checkConfigCmd struct {
	file string
}

func newCheckConfigCmd() *cobra.Command {
	b := new(checkConfigCmd)
	c := &cobra.Command{
		Use:                   "checkconfig [-file FILE]",
		Short:                 "Check the config file syntax",
		Long:                  `Validate the local YourBase config file, .yourbase.yml by default.`,
		Args:                  cobra.NoArgs,
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.run(cmd.Context())
		},
	}
	c.Flags().StringVar(&b.file, "file", types.MANIFEST_FILE, "YAML file to check")
	return c
}

func (b *checkConfigCmd) run(ctx context.Context) error {
	targetPackage, err := GetTargetPackageNamed(b.file)
	if err != nil {
		return err
	}

	log.Infof(ctx, "Syntax for package '%s' is OK: your package is YourBase'd!", targetPackage.Name)
	return nil
}
