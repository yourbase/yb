package config

import (
	"fmt"
	"os"
	"strings"
)

type IsolationType int

const (
	IsolationContainers IsolationType = 0
	IsolationVMs        IsolationType = 1
)

const (
	APP_SETTINGS       = "/user/settings"
	API_TOKEN_VALIDATE = "/apikey/validate/%s"
)

// TODO per-project?
func Isolation() IsolationType {
	return IsolationContainers
}

func ShouldUploadBuildLogs() bool {

	if v, err := GetConfigValue("user", "upload_build_logs"); err == nil {
		if v == "true" {
			return true
		}
	}

	return false
}

func YourBaseProfile() string {
	profile, exists := os.LookupEnv("YOURBASE_PROFILE")

	if exists {
		return profile
	}

	profile, exists = os.LookupEnv("YB_PROFILE")

	if exists {
		return profile
	}

	profile, err := GetConfigValue("defaults", "environment")
	if err == nil {
		return profile
	}

	return ""
}

func apiBaseUrl() (string, error) {
	if url, exists := os.LookupEnv("YOURBASE_API_URL"); exists {
		return url, nil
	}

	if url, err := GetConfigValue("yourbase", "api_url"); err == nil {
		if url != "" {
			return url, nil
		}
	}

	profile := YourBaseProfile()

	switch profile {
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
		return "", fmt.Errorf("Unknown environment (%s) and no override in the config file or environment available", profile)
	}

	return "", fmt.Errorf("Unable to generate URL")
}

func ApiUrl(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	if baseUrl, err := apiBaseUrl(); err != nil {
		return "", fmt.Errorf("Can't determine API URL: %v", err)
	} else {
		return fmt.Sprintf("%s%s", baseUrl, path), nil
	}
}

func TokenValidationUrl(apiToken string) (string, error) {
	return ApiUrl(fmt.Sprintf(API_TOKEN_VALIDATE, apiToken))
}

func managementBaseUrl() (string, error) {
	if url, exists := os.LookupEnv("YOURBASE_UI_URL"); exists {
		return url, nil
	}

	if url, err := GetConfigValue("yourbase", "management_url"); err == nil {
		if url != "" {
			return url, nil
		}
	}

	profile := YourBaseProfile()

	switch profile {
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
		return "", fmt.Errorf("Unknown environment (%s) and no override in the config file or environment available", profile)
	}

	return "", fmt.Errorf("Unable to generate URL")
}

func ManagementUrl(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	if baseUrl, err := managementBaseUrl(); err != nil {
		return "", fmt.Errorf("Couldn't determine management URL: %v", err)
	} else {
		return fmt.Sprintf("%s%s", baseUrl, path), nil
	}
}

func UserSettingsUrl() (string, error) {
	return ManagementUrl(APP_SETTINGS)
}

func CurrentGHAppUrl() (gh string) {
	profile := YourBaseProfile()

	switch profile {
	case "staging":
		gh = "https://github.com/apps/yourbase-staging"
	case "preview":
		gh = "https://github.com/apps/yourbase-preview"
	case "development":
		gh = "https://github.com/apps/yourbase-development"
	case "production":
		gh = "https://github.com/apps/yourbase"
	default:
		gh = "https://github.com/apps/yourbase"
	}

	return
}

func UserToken() (string, error) {
	token, exists := os.LookupEnv("YB_USER_TOKEN")
	if !exists {
		token, err := GetConfigValue("user", "api_key")

		if err != nil {
			return "", fmt.Errorf("Unable to find YB token in config file or environment.\nUse yb login to fetch one, if you already logged in to https://app.yourbase.io")
		}

		return token, nil
	} else {
		return token, nil
	}

	return "", fmt.Errorf("Unable to determine token - not in the config or environment")
}
