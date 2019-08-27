package config

import (
	"fmt"
	"os"
	"strings"
)

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
		return "http://api.preview.yourbase.io:5000", nil
	case "development":
		return "http://localhost:5000", nil
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
		return "http://app.preview.yourbase.io:3000", nil
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
