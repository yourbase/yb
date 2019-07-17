package plumbing

import (
	"fmt"
	"gopkg.in/ini.v1"
	"os"
	"os/user"
	"path/filepath"
)

func configFilePath() string {
	u, _ := user.Current()
	configDir := filepath.Join(u.HomeDir, ".config", "yb")
	MkdirAsNeeded(configDir)
	iniPath := filepath.Join(configDir, "settings.ini")

	if !PathExists(iniPath) {
		emptyFile, _ := os.Create(iniPath)
		emptyFile.Close()
	}

	return iniPath
}

func loadConfigFile() (*ini.File, error) {
	iniPath := configFilePath()
	cfg, err := ini.Load(iniPath)

	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}

	return cfg, nil

}

func GetConfigValue(section string, key string) (string, error) {
	if cfg, err := loadConfigFile(); err != nil {
		return "", err
	} else {
		return cfg.Section(section).Key(key).String(), nil
	}
}

func SetConfigValue(section string, key string, value string) error {
	if cfg, err := loadConfigFile(); err != nil {
		return err
	} else {
		cfg.Section(section).Key(key).SetValue(value)
		iniPath := configFilePath()
		cfg.SaveTo(iniPath)
		return nil
	}
}
