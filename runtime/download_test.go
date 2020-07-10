package runtime

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go4.org/xdgdir"
)

func Test_downloadFileWithCache(t *testing.T) {
	const constantMock = "Please download me"
	var headMethodCount, getMethodCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Current request: %v\nMethod: %s", r, r.Method)
		switch r.Method {
		case "HEAD":
			headMethodCount += 1
		case "GET", "":
			getMethodCount += 1
		}
		fmt.Fprint(w, constantMock)
	}))

	defer ts.Close()
	cachePath := filepath.Join(xdgdir.Cache.Path(), "yourbase")

	cacheFilename, err := downloadFileWithCache(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cacheFilename, cachePath) {
		t.Errorf("cache filename doesn't start with %s", cachePath)
	}

	// Checks contents
	data, err := ioutil.ReadFile(cacheFilename)
	if err != nil {
		t.Fatal(err)
	}

	if constantMock != string(data) {
		t.Errorf("file contents: want '%s'; got '%s'", constantMock, data)
	}

	if headMethodCount > 0 {
		t.Errorf("expected 0 HEAD, got %d", headMethodCount)
	}
	if getMethodCount > 1 {
		t.Errorf("expected 1 GET, got: %d", getMethodCount)
	}

	// Try again the same URL, should hit cache
	cacheFilename, err = downloadFileWithCache(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cacheFilename, cachePath) {
		t.Errorf("cache filename doesn't start with %s", cachePath)
	}

	// Checks contents
	data, err = ioutil.ReadFile(cacheFilename)
	if err != nil {
		t.Fatal(err)
	}

	if constantMock != string(data) {
		t.Errorf("file contents: want %s; got %s", constantMock, data)
	}

	if headMethodCount > 1 {
		t.Errorf("expected 1 HEAD, got: %d", headMethodCount)
	}
	if getMethodCount > 1 {
		t.Errorf("expected 1 GET, got: %d", getMethodCount)
	}

	if err := os.Remove(cacheFilename); err != nil {
		t.Fatal(err)
	}

}
