package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/matishsiao/goInfo"
	"github.com/spf13/cobra"
	"github.com/yourbase/commons/envvar"
	ybconfig "github.com/yourbase/yb/internal/config"
	"zombiezen.com/go/log"
)

var (
	version   string = "DEVELOPMENT"
	channel   string = "development"
	date      string
	commitSHA string
)

func versionString() string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, version)
	fmt.Fprintln(sb, "Channel:", channel)
	if date != "" {
		fmt.Fprintln(sb, "Date:", date)
	}
	if commitSHA != "" {
		fmt.Fprintln(sb, "Commit:", commitSHA)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func main() {
	rootCmd := &cobra.Command{
		Use:           "yb",
		Short:         "Build tool optimized for local + remote development",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       versionString(),
	}
	showDebug := rootCmd.PersistentFlags().Bool("debug", false, "show debug logs")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		initLog(*showDebug)
		displayOldDirectoryWarning(cmd.Context())
	}

	rootCmd.AddCommand(
		newBuildCmd(),
		newCheckConfigCmd(),
		newCleanCmd(),
		newConfigCmd(),
		newExecCmd(),
		newGenCompleteCmd(),
		newInitCmd(),
		newLoginCmd(),
		newRemoteCmd(),
		newRunCmd(),
		newTokenCmd(),
	)
	rootCmd.AddCommand(&cobra.Command{
		Use:           "platform",
		Short:         "Show platform information",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			gi := goInfo.GetInfo()
			gi.VarDump()
		},
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:           "version",
		Short:         "Show version info",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("yb version", rootCmd.Version)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	setupSignals(cancel)
	err := rootCmd.ExecuteContext(ctx)
	cancel()
	if err != nil {
		initLog(false)
		log.Errorf(ctx, "%v", err)
		os.Exit(1)
	}
}

func displayOldDirectoryWarning(ctx context.Context) {
	home := os.Getenv("HOME")
	if home == "" {
		return
	}
	dir := filepath.Join(home, ".yourbase")
	if _, err := os.Stat(dir); err == nil {
		log.Warnf(ctx, "yb no longer uses %s to store files. You can remove this directory to save disk space.", dir)
	}
}

type logger struct {
	color      bool
	showLevels bool

	mu  sync.Mutex
	buf []byte
}

var logInitOnce sync.Once

func initLog(showDebug bool) {
	logInitOnce.Do(func() {
		log.SetDefault(&log.LevelFilter{
			Min: configuredLogLevel(showDebug),
			Output: &logger{
				color:      colorLogs(),
				showLevels: showLogLevels(),
			},
		})
	})
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

func configuredLogLevel(showDebug bool) log.Level {
	if showDebug {
		return log.Debug
	}
	l, _ := ybconfig.Get("defaults", "log-level")
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
	out, _ := ybconfig.Get("defaults", "no-pretty-output")
	if out != "" {
		b, _ := strconv.ParseBool(out)
		return !b
	}
	return !envvar.Bool("YB_NO_PRETTY_OUTPUT")
}
