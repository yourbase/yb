package main

import (
	"context"
	"flag"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/johnewart/subcommands"
	"github.com/yourbase/commons/envvar"
	"github.com/yourbase/yb/cli"
	ybconfig "github.com/yourbase/yb/config"
	"zombiezen.com/go/log"
)

var (
	version   string = "DEVELOPMENT"
	channel   string = "development"
	date      string
	commitSHA string
)

func main() {
	log.SetDefault(&log.LevelFilter{
		Min: configuredLogLevel(),
		Output: &logger{
			color:      colorLogs(),
			showLevels: showLogLevels(),
		},
	})

	cmdr := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))
	cmdr.Register(cmdr.HelpCommand(), "")
	cmdr.Register(cmdr.FlagsCommand(), "")
	cmdr.Register(cmdr.CommandsCommand(), "")
	cmdr.Register(&cli.BuildCmd{Version: version, Channel: channel, CommitSHA: commitSHA}, "")
	cmdr.Register(&cli.CheckConfigCmd{}, "")
	cmdr.Register(&cli.ConfigCmd{}, "")
	cmdr.Register(&cli.ExecCmd{}, "")
	cmdr.Register(&cli.LoginCmd{}, "")
	cmdr.Register(&cli.PlatformCmd{}, "")
	cmdr.Register(&cli.RemoteCmd{}, "")
	cmdr.Register(&cli.RunCmd{}, "")
	cmdr.Register(&cli.UpdateCmd{}, "")
	cmdr.Register(&cli.WorkspaceCmd{}, "")
	cmdr.Register(&cli.VersionCmd{Version: version, Channel: channel, Date: date, CommitSHA: commitSHA}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(cmdr.Execute(ctx)))
}

type logger struct {
	color      bool
	showLevels bool

	mu  sync.Mutex
	buf []byte
}

func (l *logger) Log(ctx context.Context, entry log.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = l.buf[:0]
	if l.showLevels {
		if l.color {
			switch {
			case entry.Level >= log.Error:
				// Red text
				l.buf = append(l.buf, "\x1b[31m"...)
			case entry.Level >= log.Warn:
				// Yellow text
				l.buf = append(l.buf, "\x1b[33m"...)
			default:
				// Cyan text
				l.buf = append(l.buf, "\x1b[36m"...)
			}
		}
		switch {
		case entry.Level >= log.Error:
			l.buf = append(l.buf, "ERROR"...)
		case entry.Level >= log.Warn:
			l.buf = append(l.buf, "WARN"...)
		case entry.Level >= log.Info:
			l.buf = append(l.buf, "INFO"...)
		default:
			l.buf = append(l.buf, "DEBUG"...)
		}
		if l.color {
			l.buf = append(l.buf, "\x1b[0m"...)
		}
		l.buf = append(l.buf, ' ')
	}
	l.buf = append(l.buf, entry.Msg...)
	l.buf = append(l.buf, '\n')
	os.Stderr.Write(l.buf)
}

func (l *logger) LogEnabled(entry log.Entry) bool {
	return true
}

func configuredLogLevel() log.Level {
	l, _ := ybconfig.GetConfigValue("defaults", "log-level")
	switch strings.ToLower(l) {
	case "debug":
		return log.Debug
	case "warn", "warning":
		return log.Warn
	case "error":
		return log.Error
	}
	return log.Info
}

func colorLogs() bool {
	b, _ := strconv.ParseBool(envvar.Get("CLICOLOR", "1"))
	return b
}

func showLogLevels() bool {
	out, _ := ybconfig.GetConfigValue("defaults", "no-pretty-output")
	if out != "" {
		b, _ := strconv.ParseBool(out)
		return !b
	}
	return !envvar.Bool("YB_NO_PRETTY_OUTPUT")
}
