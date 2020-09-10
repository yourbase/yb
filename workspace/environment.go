package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/joho/godotenv"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

// checkAndSplitEnvVar returns two empty strings and false
// if env isn't formed as "something=.*"
func checkAndSplitEnvVar(env string) (name, value string, sane bool) {
	s := strings.SplitN(env, "=", 2)
	if sane = (len(s) == 2); sane {
		name = s[0]
		value = s[1]
	}
	return
}

// parseEnvironment checks and process an arbitrary number of
//  environment vars lists, trying to not duplicate anything
func parseEnvironment(ctx context.Context, envPath string, runtimeData runtime.RuntimeEnvironmentData, envPacks ...[]string) ([]string, error) {
	envMap := make(map[string]string)
	for _, envPack := range envPacks {
		for _, prop := range envPack {
			interpolated, err := templateToString(prop, runtimeData)
			if err == nil {
				prop = interpolated
			} else {
				return nil, err
			}
			if key, value, ok := checkAndSplitEnvVar(prop); ok {
				envMap[key] = value
			} else {
				return nil, errors.New("invalid enviroment var spec: " + prop)
			}
		}
	}

	// Check and load .env file
	localEnv, err := godotenv.Read(envPath)
	if err == nil {
		for k, v := range localEnv {
			envMap[k] = v
		}
	} else {
		log.Debugf("Dotenv load error: %v", err)
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("processing local .env: %w", err)
		}
	}

	result := make([]string, 0)
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result, nil
}

// templateToString process data and apply passed in templateText
// returning an interpolated string
func templateToString(templateText string, data interface{}) (string, error) {
	t, err := template.New("generic").Parse(templateText)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		log.Errorf("Can't render template:: %v", err)
		return "", err
	}

	result := tpl.String()
	return result, nil
}
