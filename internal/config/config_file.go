package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/yourbase/commons/ini"
	"go4.org/xdgdir"
)

const (
	dirName      = "yb"
	settingsName = "settings.ini"
)

func Load() (ini.FileSet, error) {
	var filePaths []string
	for _, dir := range xdgdir.Config.SearchPaths() {
		filePaths = append(filePaths, filepath.Join(dir, dirName, settingsName))
	}
	cfg, err := ini.ParseFiles(nil, filePaths...)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	return cfg, nil
}

func ResolveSectionName(cfg Getter, name string) string {
	if name == "defaults" {
		return name
	}
	if profile := getProfile(cfg); profile != "" {
		return profile + ":" + name
	}
	return name
}

// Getter is the interface that wraps the Get method on *ini.File and ini.FileSet.
type Getter interface {
	Get(section, key string) string
}

func Get(cfg Getter, section string, key string) string {
	return cfg.Get(ResolveSectionName(cfg, section), key)
}

func Save(cfg *ini.File) error {
	configRoot := xdgdir.Config.Path()
	if configRoot == "" {
		return fmt.Errorf("save configuration: %v not set", xdgdir.Config)
	}
	data, err := cfg.MarshalText()
	if err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}
	primaryPath := filepath.Join(configRoot, dirName, settingsName)
	if err := os.MkdirAll(filepath.Dir(primaryPath), 0777); err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}
	if err := ioutil.WriteFile(primaryPath, data, 0666); err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}
	return nil
}
