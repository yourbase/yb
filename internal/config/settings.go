package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

func ShouldUploadBuildLogs() bool {
	if v, err := Get("user", "upload_build_logs"); err == nil {
		if v == "true" {
			return true
		}
	}
	return false
}

func getProfile(cfg *ini.File) string {
	if profile := os.Getenv("YOURBASE_PROFILE"); profile != "" {
		return profile
	}
	if profile := os.Getenv("YB_PROFILE"); profile != "" {
		return profile
	}
	return cfg.Section("defaults").Key("environment").String()
}

func apiBaseURL() (*url.URL, error) {
	if u := os.Getenv("YOURBASE_API_URL"); u != "" {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("determine API URL: %w", err)
		}
		return parsed, nil
	}
	cfg, err := loadConfigFiles()
	if err != nil {
		return nil, fmt.Errorf("determine API URL: %w", err)
	}
	if u := get(cfg, "yourbase", "api_url"); u != "" {
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

func APIURL(path string) (*url.URL, error) {
	base, err := apiBaseURL()
	if err != nil {
		return nil, err
	}
	return appendURLPath(base, path), nil
}

func TokenValidationURL(apiToken string) (*url.URL, error) {
	return APIURL("/apikey/validate/" + apiToken)
}

func uiBaseURL() (*url.URL, error) {
	if u := os.Getenv("YOURBASE_UI_URL"); u != "" {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("determine UI URL: %w", err)
		}
		return parsed, nil
	}
	cfg, err := loadConfigFiles()
	if err != nil {
		return nil, fmt.Errorf("determine UI URL: %w", err)
	}
	if u := get(cfg, "yourbase", "management_url"); u != "" {
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

func UIURL(path string) (*url.URL, error) {
	base, err := uiBaseURL()
	if err != nil {
		return nil, err
	}
	return appendURLPath(base, path), nil
}

func UserSettingsURL() (*url.URL, error) {
	return UIURL("/user/settings")
}

func GitHubAppURL() *url.URL {
	profile := "production"
	if cfg, err := loadConfigFiles(); err == nil {
		profile = getProfile(cfg)
	}
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

func UserToken() (string, error) {
	if token := os.Getenv("YB_USER_TOKEN"); token != "" {
		return token, nil
	}
	token, err := Get("user", "api_key")
	if err != nil {
		return "", fmt.Errorf("get API token: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("get API token: not found in configuration or environment. " +
			"Use 'yb login' to log into YourBase.")
	}
	return token, nil
}
