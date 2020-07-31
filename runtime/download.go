package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/yourbase/yb/plumbing/log"
	"go4.org/xdgdir"
)

const defaultCacheDir = "/tmp/yourbase/cache"

func localCacheDir() string {
	if cacheDir, exists := os.LookupEnv("YB_CACHE_DIR"); exists {
		return cacheDir
	}
	// Tries to find a XDG Cache dir
	if cacheDir := xdgdir.Cache.Path(); cacheDir != "" {
		return filepath.Join(cacheDir, "yourbase")
	}
	return defaultCacheDir
}

func cacheFilenameForURL(url string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9.]+")
	if err != nil {
		return "", fmt.Errorf("compiling regex: %v", err)
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

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("creating dir %s: %v", cacheDir, err)
	}

	cacheFilename := filepath.Join(cacheDir, filename)
	log.Infof("Downloading %s to cache as %s", url, cacheFilename)

	// Exists, don't re-download
	if fi, err := os.Stat(cacheFilename); err == nil {
		// Cancellable request
		req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
		if err != nil {
			return "", err
		}

		// try HEAD'ing the URL and comparing to local file
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("fetching %s: %v", url, err)
		}
		if err := resp.Body.Close(); err != nil {
			// Non fatal
			log.Warnf("Trying to close response body: %v", err)
		}
		// checks response HTTP status
		// TODO add support for retrying and resuming partial downloads
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s status %s", url, resp.Status)
		}
		if fi.Size() == resp.ContentLength {
			log.Infof("Re-using cached version of %s", url)
			return cacheFilename, nil
		}

		// TODO add checksum validation

		log.Infof("Re-downloading %s because remote file and local file differ in size", url)

	}

	// Otherwise download
	err = doDownload(ctx, cacheFilename, url)
	return cacheFilename, err
}

// doDownload uses a named err to better control edge cases of async writes to the disk
func doDownload(ctx context.Context, filepath string, url string) (err error) {

	// Cancellable request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	// Get the data
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		cerr := resp.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return
}
