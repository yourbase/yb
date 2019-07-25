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

	return ""
}

func apiBaseUrl() string {
	if url, exists := os.LookupEnv("YOURBASE_API_URL"); exists {
		return url
	}

	if url, err := GetConfigValue("yourbase", "api_url"); err == nil {
		if url != "" {
			return url
		}
	}

	profile := YourBaseProfile()

	switch profile {
	case "staging":
		return "https://api.staging.yourbase.io"
	case "development":
		return "http://localhost:5000"
	default:
		return "https://api.yourbase.io"
	}
}

func ApiUrl(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	apiURL := fmt.Sprintf("%s%s", apiBaseUrl(), path)

	return apiURL
}

func managementBaseUrl() string {
	if url, exists := os.LookupEnv("YOURBASE_UI_URL"); exists {
		return url
	}

	if url, err := GetConfigValue("yourbase", "management_url"); err == nil {
		if url != "" {
			return url
		}
	}

	profile := YourBaseProfile()

	switch profile {
	case "staging":
		return "https://app.staging.yourbase.io"
	case "development":
		return "http://localhost:3000"
	default:
		return "https://app.yourbase.io"
	}
}

func ManagementUrl(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	managementURL := fmt.Sprintf("%s%s", managementBaseUrl(), path)

	return managementURL
}

func UserToken() (string, error) {
	token, exists := os.LookupEnv("YB_USER_TOKEN")
	if !exists {
		if token, err := GetConfigValue("user", "api_key"); err != nil {
			return "", fmt.Errorf("Unable to find YB token in config file or environment.\nUse yb login to fetch one, if you already logged in to https://app.yourbase.io")
		} else {
			fmt.Printf("User token: %s\n", token)
			return token, nil
		}
	} else {
		return token, nil
	}
}
