package config

import (
	"fmt"
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

func apiBaseURL() (string, error) {
	if url, exists := os.LookupEnv("YOURBASE_API_URL"); exists {
		return url, nil
	}
	cfg, err := loadConfigFiles()
	if err != nil {
		return "", fmt.Errorf("determine API URL: %w", err)
	}
	if url := get(cfg, "yourbase", "api_url"); url != "" {
		return url, nil
	}
	switch profile := getProfile(cfg); profile {
	case "staging":
		return "https://api.staging.yourbase.io", nil
	case "preview":
		return "https://api.preview.yourbase.io", nil
	case "development":
		return "http://localhost:5001", nil
	case "production":
		return "https://api.yourbase.io", nil
	case "":
		return "https://api.yourbase.io", nil
	default:
		return "", fmt.Errorf("determine API URL: unknown environment %s and no configuration set", profile)
	}
}

func APIURL(path string) (string, error) {
	base, err := apiBaseURL()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/"), nil
}

func TokenValidationURL(apiToken string) (string, error) {
	return APIURL("/apikey/validate/" + apiToken)
}

func uiBaseURL() (string, error) {
	if url := os.Getenv("YOURBASE_UI_URL"); url != "" {
		return url, nil
	}
	cfg, err := loadConfigFiles()
	if err != nil {
		return "", fmt.Errorf("determine UI URL: %w", err)
	}
	if url := get(cfg, "yourbase", "management_url"); url != "" {
		return url, nil
	}
	switch profile := getProfile(cfg); profile {
	case "staging":
		return "https://app.staging.yourbase.io", nil
	case "preview":
		return "https://app.preview.yourbase.io", nil
	case "development":
		return "http://localhost:3000", nil
	case "production":
		return "https://app.yourbase.io", nil
	case "":
		return "https://app.yourbase.io", nil
	default:
		return "", fmt.Errorf("determine UI URL: unknown environment %s and no configuration set", profile)
	}
}

func UIURL(path string) (string, error) {
	base, err := uiBaseURL()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/"), nil
}

func UserSettingsURL() (string, error) {
	return UIURL("/user/settings")
}

func GitHubAppURL() (gh string) {
	profile := "production"
	if cfg, err := loadConfigFiles(); err == nil {
		profile = getProfile(cfg)
	}
	switch profile {
	case "staging":
		return "https://github.com/apps/yourbase-staging"
	case "preview":
		return "https://github.com/apps/yourbase-preview"
	case "development":
		return "https://github.com/apps/yourbase-development"
	default:
		return "https://github.com/apps/yourbase"
	}
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
