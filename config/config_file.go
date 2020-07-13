package config

import (
	"fmt"
	"gopkg.in/ini.v1"
	"os"
	"os/user"
	"path/filepath"
)

func configFilePath() (string, error) {
	u, err := user.Current()

	if err != nil {
		return "", err
	}

	configDir := filepath.Join(u.HomeDir, ".config", "yb")
	mkdirAsNeeded(configDir)
	iniPath := filepath.Join(configDir, "settings.ini")

	if !pathExists(iniPath) {
		emptyFile, _ := os.Create(iniPath)
		emptyFile.Close()
	}

	return iniPath, nil
}

func loadConfigFile() (*ini.File, error) {
	iniPath, err := configFilePath()
	if err != nil {
		return nil, fmt.Errorf("determining config file path: %v", err)
	}

	cfg, err := ini.Load(iniPath)

	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}

	return cfg, nil

}

// To break an import cycle
func pathExists(path string) bool {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return false
	}

	return true
}

// To break an import cycle
func mkdirAsNeeded(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("creating dir: %v", err)
		}
	}

	return nil
}

func SectionPrefix() string {
	profile := YourBaseProfile()
	if profile != "" {
		return fmt.Sprintf("%s:", profile)
	}

	return ""
}

func GetConfigValue(section string, key string) (string, error) {
	var sectionPrefix string
	if section != "defaults" {
		sectionPrefix = SectionPrefix()
	}
	cfgSection := fmt.Sprintf("%s%s", sectionPrefix, section)

	if cfg, err := loadConfigFile(); err != nil {
		return "", err
	} else {
		return cfg.Section(cfgSection).Key(key).String(), nil
	}
}

func SetConfigValue(section string, key string, value string) error {
	var sectionPrefix string
	if section != "defaults" {
		sectionPrefix = SectionPrefix()
	}
	cfgSection := fmt.Sprintf("%s%s", sectionPrefix, section)

	if cfg, err := loadConfigFile(); err != nil {
		return err
	} else {
		cfg.Section(cfgSection).Key(key).SetValue(value)
		if iniPath, err := configFilePath(); err != nil {
			return err
		} else {
			cfg.SaveTo(iniPath)
		}
		return nil
	}
}
