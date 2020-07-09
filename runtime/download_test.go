package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"go4.org/xdgdir"
)

func Test_downloadFileWithCache(t *testing.T) {
	contantMock := "Please download me"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, contantMock)
	}))

	defer ts.Close()
	cachePath := filepath.Join(xdgdir.Cache.Path(), "yourbase")

	cacheFilename, err := downloadFileWithCache(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if cacheFilename == "" {
		t.Error("got an empty sring instead of a cache filename")
	}
	if !strings.HasPrefix(cacheFilename, cachePath) {
		t.Errorf("cache filename doesn't start with %s", cachePath)
	}

	// Try again the same URL, should hit cache
	cacheFilename, err = downloadFileWithCache(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if cacheFilename == "" {
		t.Error("got an empty sring instead of a cache filename")
	}
	if !strings.HasPrefix(cacheFilename, cachePath) {
		t.Errorf("cache filename doesn't start with %s", cachePath)
	}
}
