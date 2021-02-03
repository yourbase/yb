package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func ShouldUploadBuildLogs(cfg Getter) bool {
	v, err := strconv.ParseBool(Get(cfg, "user", "upload_build_logs"))
	if err != nil {
		return false
	}
	return v
}

func getProfile(cfg Getter) string {
	if profile := os.Getenv("YOURBASE_PROFILE"); profile != "" {
		return profile
	}
	if profile := os.Getenv("YB_PROFILE"); profile != "" {
		return profile
	}
	return cfg.Get("defaults", "environment")
}

func apiBaseURL(cfg Getter) (*url.URL, error) {
	if u := os.Getenv("YOURBASE_API_URL"); u != "" {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("determine API URL: %w", err)
		}
		return parsed, nil
	}
	if u := Get(cfg, "yourbase", "api_url"); u != "" {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("determine API URL: %w", err)
		}
		return parsed, nil
	}
	switch profile := getProfile(cfg); profile {
	case "staging":
		return &url.URL{
			Scheme: "https",
			Host:   "api.staging.yourbase.io",
		}, nil
	case "preview":
		return &url.URL{
			Scheme: "https",
			Host:   "api.preview.yourbase.io",
		}, nil
	case "development":
		return &url.URL{
			Scheme: "https",
			Host:   "localhost:5001",
		}, nil
	case "", "production":
		return &url.URL{
			Scheme: "https",
			Host:   "api.yourbase.io",
		}, nil
	default:
		return nil, fmt.Errorf("determine API URL: unknown environment %s and no configuration set", profile)
	}
}

func APIURL(cfg Getter, path string) (*url.URL, error) {
	base, err := apiBaseURL(cfg)
	if err != nil {
		return nil, err
	}
	return appendURLPath(base, path), nil
}

func TokenValidationURL(cfg Getter) (*url.URL, error) {
	return APIURL(cfg, "/users/whoami")
}

func uiBaseURL(cfg Getter) (*url.URL, error) {
	if u := os.Getenv("YOURBASE_UI_URL"); u != "" {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("determine UI URL: %w", err)
		}
		return parsed, nil
	}
	if u := Get(cfg, "yourbase", "management_url"); u != "" {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("determine UI URL: %w", err)
		}
		return parsed, nil
	}
	switch profile := getProfile(cfg); profile {
	case "staging":
		return &url.URL{
			Scheme: "https",
			Host:   "app.staging.yourbase.io",
		}, nil
	case "preview":
		return &url.URL{
			Scheme: "https",
			Host:   "app.preview.yourbase.io",
		}, nil
	case "development":
		return &url.URL{
			Scheme: "https",
			Host:   "localhost:3000",
		}, nil
	case "", "production":
		return &url.URL{
			Scheme: "https",
			Host:   "app.yourbase.io",
		}, nil
	default:
		return nil, fmt.Errorf("determine UI URL: unknown environment %s and no configuration set", profile)
	}
}

func UIURL(cfg Getter, path string) (*url.URL, error) {
	base, err := uiBaseURL(cfg)
	if err != nil {
		return nil, err
	}
	return appendURLPath(base, path), nil
}

func UserSettingsURL(cfg Getter) (*url.URL, error) {
	return UIURL(cfg, "/user/settings")
}

func GitHubAppURL() *url.URL {
	profile := "production"
	appName := "yourbase"
	switch profile {
	case "staging":
		appName = "yourbase-staging"
	case "preview":
		appName = "yourbase-preview"
	case "development":
		appName = "yourbase-development"
	}
	return &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/apps/" + appName,
	}
}

func appendURLPath(u *url.URL, path string) *url.URL {
	u2 := new(url.URL)
	*u2 = *u
	u2.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	return u2
}

func UserToken(cfg Getter) (string, error) {
	if token := os.Getenv("YB_USER_TOKEN"); token != "" {
		return token, nil
	}
	token := Get(cfg, "user", "api_key")
	if token == "" {
		return "", fmt.Errorf("get API token: not found in configuration or environment. " +
			"Use 'yb login' to log into YourBase.")
	}
	return token, nil
}
