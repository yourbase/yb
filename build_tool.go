package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

type BuildTool interface {
	Install() error
	Setup() error
	Version() string
	Instructions() BuildInstructions
}

func CacheDir() string {
	cacheDir := "/home/jewart/.artificer/cache"
	MkdirAsNeeded(cacheDir)
	return cacheDir
}

func CacheFilenameForUrl(url string) (string, error) {
	cacheDir := CacheDir()
	reg, err := regexp.Compile("[^a-zA-Z0-9.]+")
	if err != nil {
		return "", fmt.Errorf("Can't compile regex: %v", err)
	}

	fileName := reg.ReplaceAllString(url, "")
	return filepath.Join(cacheDir, fileName), nil
}

func DownloadFileWithCache(url string) (string, error) {
	cacheFilename, err := CacheFilenameForUrl(url)

	if err != nil {
		return cacheFilename, err
	}

	// Exists, don't re-download
	if _, err := os.Stat(cacheFilename); !os.IsNotExist(err) {
		fmt.Printf("Cached version of %s already downloaded as %s, skipping!\n", url, cacheFilename)
		return cacheFilename, nil
	}

	err = DownloadFile(cacheFilename, url)

	if err != nil {
		return cacheFilename, err
	}

	return cacheFilename, nil
}

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func OS() string {
	return "linux"
}

func Arch() string {
	return "amd64"
}
