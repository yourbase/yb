package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/matishsiao/goInfo"
	"github.com/spf13/cobra"
	"github.com/yourbase/yb/internal/config"
	"zombiezen.com/go/log"
)

var (
	version   string = "DEVELOPMENT"
	channel   string = "development"
	date      string
	commitSHA string
)

func versionString() string {
	gi := goInfo.GetInfo()
	sb := new(strings.Builder)
	fmt.Fprintln(sb, version)
	fmt.Fprintln(sb, "Channel:", channel)
	fmt.Fprintf(sb, "Host OS: %s/%s %s (%d cores)\n", runtime.GOOS, runtime.GOARCH, gi.Core, runtime.NumCPU())
	if date != "" {
		fmt.Fprintln(sb, "Date:", date)
	}
	if commitSHA != "" {
		fmt.Fprintln(sb, "Commit:", commitSHA)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	setupSignals(cancel)
	cfg, err := config.Load()
	if err != nil {
		initLog(cfg, false)
		log.Errorf(ctx, "%v", err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:           "yb",
		Short:         "Build tool optimized for local + remote development",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       versionString(),
	}
	showDebug := rootCmd.PersistentFlags().Bool("debug", false, "show debug logs")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		initLog(cfg, *showDebug)
		displayOldDirectoryWarning(cmd.Context())
	}

	rootCmd.AddCommand(
		newBuildCmd(),
		newCheckConfigCmd(),
		newCleanCmd(),
		newConfigCmd(cfg),
		newExecCmd(),
		newGenCompleteCmd(),
		newInitCmd(),
		newLoginCmd(cfg),
		newRemoteCmd(cfg),
		newRunCmd(),
		newTokenCmd(cfg),
	)
	rootCmd.AddCommand(&cobra.Command{
		Use:           "version",
		Short:         "Show version information",
		Aliases:       []string{"platform"},
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("yb version", rootCmd.Version)
		},
	})

	err = rootCmd.ExecuteContext(ctx)
	cancel()
	if err != nil {
		initLog(cfg, false)
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
