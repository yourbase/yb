package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	ybconfig "github.com/yourbase/yb/internal/config"
	"zombiezen.com/go/log"
)

type loginCmd struct {
}

func newLoginCmd() *cobra.Command {
	b := new(loginCmd)
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
	reader := bufio.NewReader(os.Stdin)
	tokenURL, err := ybconfig.UserSettingsURL()
	if err != nil {
		return err
	}

	tokenPrompt := fmt.Sprintf("Open up %s and then paste the token here.", tokenURL)
	fmt.Println(tokenPrompt)
	fmt.Print("API Token: ")
	apiToken, _ := reader.ReadString('\n')
	apiToken = strings.TrimSuffix(apiToken, "\n")

	// Using "/users/whoami" to validate the apikey
	validationURL, err := ybconfig.TokenValidationURL()
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

	if err := ybconfig.Set("user", "api_key", apiToken); err != nil {
		return fmt.Errorf("store token: %w", err)
	}
	log.Infof(ctx, "API token saved to the config file")
	return nil
}
