package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"go4.org/xdgdir"
	"gopkg.in/ini.v1"
	"zombiezen.com/go/log"
)

const (
	dirName      = "yb"
	settingsName = "settings.ini"
)

func loadConfigFiles() (*ini.File, error) {
	// xdgdir.Config.SearchPaths returns config files in descending order of preference,
	// but ini.Load accepts them in ascending order of preference, so gather them in reverse.
	searchPaths := xdgdir.Config.SearchPaths()
	var iniFiles []interface{}
	for i := len(searchPaths) - 1; i >= 0; i-- {
		data, err := ioutil.ReadFile(filepath.Join(searchPaths[i], dirName, settingsName))
		if err != nil && !os.IsNotExist(err) {
			log.Warnf(context.TODO(), "Load configuration: %v", err)
			continue
		}
		iniFiles = append(iniFiles, data)
	}
	if len(iniFiles) == 0 {
		iniFiles = append(iniFiles, []byte(nil))
	}

	cfg, err := ini.Load(iniFiles[0], iniFiles[1:]...)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	return cfg, nil
}

func resolveSectionName(cfg *ini.File, name string) string {
	if name == "defaults" {
		return name
	}
	if profile := getProfile(cfg); profile != "" {
		return profile + ":" + name
	}
	return name
}

func Get(section string, key string) (string, error) {
	cfg, err := loadConfigFiles()
	if err != nil {
		return "", err
	}
	return get(cfg, section, key), nil
}

func get(cfg *ini.File, section, key string) string {
	return cfg.Section(resolveSectionName(cfg, section)).Key(key).String()
}

func Set(section string, key string, value string) error {
	configRoot := xdgdir.Config.Path()
	if configRoot == "" {
		return fmt.Errorf("set configuration value: %v not set", xdgdir.Config)
	}
	primaryPath := filepath.Join(configRoot, dirName, settingsName)
	cfg, err := ini.LooseLoad(primaryPath)
	if err != nil {
		return fmt.Errorf("set configuration value: %w", err)
	}
	cfg.Section(resolveSectionName(cfg, section)).Key(key).SetValue(value)
	if err := os.MkdirAll(filepath.Dir(primaryPath), 0777); err != nil {
		return fmt.Errorf("set configuration value: %w", err)
	}
	if err := cfg.SaveTo(primaryPath); err != nil {
		return fmt.Errorf("set configuration value: %w", err)
	}
	return nil
}
