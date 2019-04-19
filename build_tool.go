package main

import (
	"fmt"
	"io"
	//"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	//"strconv"
	//"time"
)

type BuildTool interface {
	Install() error
	Setup() error
	Version() string
}

func ToolsDir() string {
	toolsDir, exists := os.LookupEnv("YB_TOOLS_DIR")
	if !exists {
		u, err := user.Current()
		if err != nil {
			toolsDir = "/tmp/yourbase/tools"
		} else {
			toolsDir = fmt.Sprintf("%s/.yourbase/tools", u.HomeDir)
		}
	}

	MkdirAsNeeded(toolsDir)

	return toolsDir
}

func CacheDir() string {
	cacheDir, exists := os.LookupEnv("YB_CACHE_DIR")
	if !exists {
		u, err := user.Current()
		if err != nil {
			cacheDir = "/tmp/yourbase/cache"
		} else {
			cacheDir = fmt.Sprintf("%s/.yourbase/cache", u.HomeDir)
		}
	}

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

/*
func PrintDownloadPercent(done chan int64, path string, total int64) {

	var stop bool = false

	for {
		select {
		case <-done:
			stop = true
		default:

			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}

			fi, err := file.Stat()
			if err != nil {
				log.Fatal(err)
			}

			size := fi.Size()

			if size == 0 {
				size = 1
			}

			var percent float64 = float64(size) / float64(total) * 100

			fmt.Printf("%.0f", percent)
			fmt.Println("%")
		}

		if stop {
			break
		}

		time.Sleep(time.Second)
	}
}

func DownloadFile(dest string, url string) error {

	log.Printf("Downloading file %s from %s\n", dest, url)

	start := time.Now()

	out, err := os.Create(dest)

	if err != nil {
		return fmt.Errorf("Unable to open destination '%s': %v\n", dest, err)
	}

	defer out.Close()

	headResp, err := http.Head(url)

	if err != nil {
		panic(err)
	}

	defer headResp.Body.Close()

	size, err := strconv.Atoi(headResp.Header.Get("Content-Length"))

	if err != nil {
		panic(err)
	}

	done := make(chan int64)

	go PrintDownloadPercent(done, dest, int64(size))

	resp, err := http.Get(url)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	n, err := io.Copy(out, resp.Body)

	if err != nil {
		panic(err)
	}

	done <- n

	elapsed := time.Since(start)
	log.Printf("Download completed in %s", elapsed)
	return nil
} */
