package cli

import (
	"context"
	"flag"
	"strings"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/config"
	"zombiezen.com/go/log"
)

var (
	VARS = []string{"environment", "log-level", "log-section", "no-pretty-output"}
)

type ConfigCmd struct {
}

func (*ConfigCmd) Name() string     { return "config" }
func (*ConfigCmd) Synopsis() string { return "Convenient way to change settings" }
func (*ConfigCmd) Usage() string {
	return `config <subcommand>`
}

func (w *ConfigCmd) SetFlags(f *flag.FlagSet) {}

func (w *ConfigCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	cmdr := subcommands.NewCommander(f, "config")
	cmdr.Register(&configSetCmd{}, "")
	cmdr.Register(&configGetCmd{}, "")
	return (cmdr.Execute(ctx))
}

type configSetCmd struct {
}

func (*configSetCmd) Name() string     { return "set" }
func (*configSetCmd) Synopsis() string { return "Change the default settings" }
func (*configSetCmd) Usage() string {
	return `set environment=<production|staging|preview|development>`
}

func (w *configSetCmd) SetFlags(f *flag.FlagSet) {
}

func (w *configSetCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) < 1 {
		log.Warnf(ctx, "%s", w.Usage())
		return subcommands.ExitFailure
	}

	if strings.Contains(f.Args()[0], "=") {
		parts := strings.Split(f.Args()[0], "=")
		if len(parts) > 1 {
			found := false
			for _, configVar := range VARS {
				if parts[0] == configVar {
					config.SetConfigValue("defaults", configVar, parts[1])
					found = true
				}
			}
			if !found {
				log.Infof(ctx, "Currently supports: '%v' config variables", strings.Join(VARS, ", "))
			}
		} else {
			log.Errorf(ctx, "Please give a full <key=value>")
			return subcommands.ExitFailure
		}
	} else {
		log.Errorf(ctx, "Please use <key=value> to set a default configuration")
		return subcommands.ExitFailure
	}

	log.Infof(ctx, "Configuration done")
	return subcommands.ExitSuccess
}

type configGetCmd struct {
}

func (*configGetCmd) Name() string     { return "get" }
func (*configGetCmd) Synopsis() string { return "Show the default settings" }
func (*configGetCmd) Usage() string {
	return `get environment`
}

func (w *configGetCmd) SetFlags(f *flag.FlagSet) {}

func (w *configGetCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) < 1 {
		log.Warnf(ctx, "%s", w.Usage())
		return subcommands.ExitFailure
	}

	found := false
	for _, configVar := range VARS {
		if passed := f.Args()[0]; passed == configVar {
			env, err := config.GetConfigValue("defaults", configVar)
			if err != nil {
				log.Errorf(ctx, "Unable to get current %v: %v", configVar, err)
				return subcommands.ExitFailure
			}
			found = true
			log.Infof(ctx, "Current %v value: '%v'", configVar, env)
		}
	}
	if !found {
		log.Infof(ctx, "Currently supports: '%v' config variables", strings.Join(VARS, ", "))
	}

	return subcommands.ExitSuccess
}
