package config

import (
	"fmt"
	"gopkg.in/ini.v1"
	"os"
	"os/user"
	"path/filepath"

	"github.com/yourbase/yb/plumbing"
)

func configFilePath() (string, error) {
	u, err := user.Current()

	if err != nil {
		return "", err
	}

	configDir := filepath.Join(u.HomeDir, ".config", "yb")
	plumbing.MkdirAsNeeded(configDir)
	iniPath := filepath.Join(configDir, "settings.ini")

	if !plumbing.PathExists(iniPath) {
		emptyFile, _ := os.Create(iniPath)
		emptyFile.Close()
	}

	return iniPath, nil
}

func loadConfigFile() (*ini.File, error) {
	iniPath, err := configFilePath()
	if err != nil {
		return nil, fmt.Errorf("Couldn't determine config file path: %v", err)
	}

	cfg, err := ini.Load(iniPath)

	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}

	return cfg, nil

}

func SectionPrefix() string {
	profile := YourBaseProfile()
	if profile != "" {
		return fmt.Sprintf("%s:", profile)
	}

	return ""
}

func GetConfigValue(section string, key string) (string, error) {
	sectionPrefix := SectionPrefix()
	cfgSection := fmt.Sprintf("%s%s", sectionPrefix, section)

	if cfg, err := loadConfigFile(); err != nil {
		return "", err
	} else {
		return cfg.Section(cfgSection).Key(key).String(), nil
	}
}

func SetConfigValue(section string, key string, value string) error {
	sectionPrefix := SectionPrefix()
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
