package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yourbase/yb/plumbing/log"
)

const defaultCacheDir = "/tmp/yourbase/cache"

func localCacheDir() string {
	if cacheDir, exists := os.LookupEnv("YB_CACHE_DIR"); exists {
		return cacheDir
	}
	// Resolve current homeDir
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return filepath.Join(u.HomeDir, ".cache", "yourbase")
	}
	// Try again
	if homeDir, exists := os.LookupEnv("HOME"); exists {
		return filepath.Join(homeDir, ".cache", "yourbase")
	}
	return defaultCacheDir
}

func cacheFilenameForURL(url string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9.]+")
	if err != nil {
		return "", fmt.Errorf("Can't compile regex: %v", err)
	}
	fileName := reg.ReplaceAllString(url, "")
	return fileName, nil
}

func downloadFileWithCache(ctx context.Context, url string) (string, error) {
	cacheDir := localCacheDir()

	filename, err := cacheFilenameForURL(url)
	if err != nil {
		return "", err
	}

	os.MkdirAll(cacheDir, 0700)

	cacheFilename := filepath.Join(cacheDir, filename)
	log.Infof("Downloading %s to cache as %s", url, cacheFilename)

	fileExists := false
	fileSizeMismatch := false

	// Exists, don't re-download
	if fi, err := os.Stat(cacheFilename); !os.IsNotExist(err) && fi != nil {
		fileExists = true

		// try HEAD'ing the URL and comparing to local file
		resp, err := http.Head(url)
		if err == nil {
			// checks response HTTP status
			// 404, 500 or others like it
			if strings.HasPrefix(resp.Status, "40") || strings.HasPrefix(resp.Status, "50") {
				return "", fmt.Errorf("%s status %s", url, resp.Status)
			}
			if fi.Size() != resp.ContentLength {
				log.Infof("Re-downloading %s because remote file and local file differ in size", url)
				fileSizeMismatch = true
			}
		} else {
			return "", fmt.Errorf("fetching %s: %v", url, err)
		}

	}

	// TODO add checksum validation

	if fileExists && !fileSizeMismatch {
		// No mismatch known, but exists, use cached version
		log.Infof("Re-using cached version of %s", url)
		return cacheFilename, nil
	}

	// Otherwise download
	err = doDownload(ctx, cacheFilename, url)
	return cacheFilename, err
}

func doDownload(ctx context.Context, filepath string, url string) error {

	// Cancellable request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{}

	// Get the data
	resp, err := client.Do(req)
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
