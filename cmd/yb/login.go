package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourbase/commons/ini"
	"github.com/yourbase/yb/internal/config"
	"go4.org/xdgdir"
	"zombiezen.com/go/log"
)

type loginCmd struct {
	cfg ini.FileSet
}

func newLoginCmd(cfg ini.FileSet) *cobra.Command {
	b := &loginCmd{
		cfg: cfg,
	}
	c := &cobra.Command{
		Use:           "login",
		Short:         "Log into YourBase",
		Long:          `Configure yb to act as you in the YourBase service.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.run(cmd.Context())
		},
	}
	return c
}

func (p *loginCmd) run(ctx context.Context) error {
	tokenURL, err := config.UserSettingsURL(p.cfg)
	if err != nil {
		return err
	}
	if len(p.cfg) == 0 {
		return fmt.Errorf("%v not set", xdgdir.Config)
	}

	reader := bufio.NewReader(os.Stdin)
	tokenPrompt := fmt.Sprintf("Open up %s and then paste the token here.", tokenURL)
	fmt.Println(tokenPrompt)
	fmt.Print("API Token: ")
	apiToken, _ := reader.ReadString('\n')
	apiToken = strings.TrimSuffix(apiToken, "\n")

	// Using "/users/whoami" to validate the apikey
	validationURL, err := config.TokenValidationURL(p.cfg)
	if err != nil {
		return err
	}
	req := &http.Request{
		Method: http.MethodGet,
		URL:    validationURL,
		Header: http.Header{
			http.CanonicalHeaderKey("YB_API_TOKEN"): {apiToken},
		},
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("make validation request: %v", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// Keep going!
	case http.StatusNotFound, http.StatusUnauthorized:
		return fmt.Errorf("invalid token provided, please check it")
	default:
		return fmt.Errorf("http %s (that's us, not you, please try again later)", resp.Status)
	}
	p.cfg.Set(config.ResolveSectionName(p.cfg, "user"), "api_key", apiToken)
	if err := config.Save(p.cfg[0]); err != nil {
		return fmt.Errorf("store token: %w", err)
	}
	log.Infof(ctx, "API token saved to the config file")
	return nil
}
