package cli

import (
	"context"
	"flag"
	"strings"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/yb/config"
	"github.com/yourbase/yb/plumbing/log"
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

func (w *configSetCmd) SetFlags(f *flag.FlagSet) {}

func (w *configSetCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) < 1 {
		log.Warnln(w.Usage())
		return subcommands.ExitFailure
	}

	if strings.Contains(f.Args()[0], "=") {
		parts := strings.Split(f.Args()[0], "=")
		if len(parts) > 1 {
			switch parts[0] {
			case "environment":
				config.SetConfigValue("defaults", "environment", parts[1])
			case "no-pretty-output":
				config.SetConfigValue("defaults", "no-pretty-output", parts[1])
			case "log-level":
				config.SetConfigValue("defaults", "log-level", parts[1])
			default:
				log.Infoln("Currently only supports 'environment' and 'no-pretty-output' config")
			}
		} else {
			log.Errorln("Please give a full <key=value>")
			return subcommands.ExitFailure
		}
	} else {
		log.Errorln("Please use <key=value> to set a default configuration")
		return subcommands.ExitFailure
	}

	log.Infoln("Configuration done")
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
		log.Warnln(w.Usage())
		return subcommands.ExitFailure
	}

	switch f.Args()[0] {
	case "environment":
		env, err := config.GetConfigValue("defaults", "environment")
		if err != nil {
			log.Errorf("Unable to get current environment: %v", err)
			return subcommands.ExitFailure
		}
		log.Infof("Current environment: '%s'", env)
	default:
		log.Infoln("Currently only supports 'environment' config")
	}

	return subcommands.ExitSuccess
}
