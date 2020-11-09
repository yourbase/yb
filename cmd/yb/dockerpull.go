// Copyright 2020 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourbase/commons/http/headers"
	"github.com/yourbase/commons/ini"
	"github.com/yourbase/yb/internal/imageref"
	"github.com/yourbase/yb/internal/ybdata"
	"zombiezen.com/go/log"
)

func pullImage(ctx context.Context, dataDirs *ybdata.Dirs, image string, auth string) (layers []string, _ error) {
	m, err := fetchImageManifest(ctx, image, auth)
	if err != nil {
		return nil, fmt.Errorf("pull %s: %w", image, err)
	}
	for _, layer := range m.Layers {
		dir, err := downloadLayer(ctx, dataDirs.Docker(), image, layer.Digest, auth)
		if err != nil {
			return nil, fmt.Errorf("pull %s: %w", image, err)
		}
		layers = append(layers, dir)
	}
	return layers, nil
}

const (
	manifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"
	layerMediaType    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	blobMediaType     = "application/octet-stream"
)

type imageManifest struct {
	Layers []imageLayer
}

type imageLayer struct {
	Size   int64
	Digest string
}

func doWithDockerAuth(ctx context.Context, client *http.Client, req *http.Request, auth string) (*http.Response, error) {
	log.Debugf(ctx, "%s %v", req.Method, req.URL)
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()

	// Get token.
	log.Debugf(ctx, "Request failed unauthenticated")
	info := strings.ReplaceAll(strings.TrimPrefix(resp.Header.Get(headers.WWWAuthenticate), "Bearer "), ",", "\n")
	details, err := ini.Parse(strings.NewReader(info), nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", headers.WWWAuthenticate, err)
	}
	tokenURL := details.Get("", "realm") + "?" + url.Values{
		"service":   {details.Get("", "service")},
		"client_id": {"zombiezen test"},
		"scope":     {details.Get("", "scope")},
	}.Encode()
	log.Debugf(ctx, "Getting token from %s", tokenURL)
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	tokenReq.Header.Set(headers.Authorization, auth)
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request: http %s", tokenResp.Status)
	}
	tokenJSON, err := ioutil.ReadAll(tokenResp.Body)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	var token struct {
		Token string
	}
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}

	// Second request, now with a Bearer token.
	log.Debugf(ctx, "Retrying %s %v with Bearer token", req.Method, req.URL)
	req = req.Clone(ctx)
	req.Header.Set(headers.Authorization, "Bearer "+token.Token)
	if req.GetBody != nil {
		req.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}
	return client.Do(req)
}

func fetchImageManifest(ctx context.Context, image string, auth string) (*imageManifest, error) {
	name, tag, digest := imageref.Parse(image)
	registry := imageref.Registry(name)
	name = strings.TrimPrefix(name, registry+"/")
	urlstr := "https://" + registry + "/v2/" + name + "/manifests/"
	if digest != "" {
		urlstr += digest
	} else if tag != "" {
		urlstr += strings.TrimPrefix(tag, ":")
	} else {
		urlstr += "latest"
	}
	req, err := http.NewRequest(http.MethodGet, urlstr, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest for %s: %w", image, err)
	}
	req.Header.Set(headers.Authorization, auth)
	req.Header.Set(headers.Accept, manifestMediaType)
	resp, err := doWithDockerAuth(ctx, http.DefaultClient, req, auth)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest for %s: %w", image, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		log.Debugf(ctx, "HTTP %s:\n%s", resp.Status, data)
		if got := resp.Header.Get(headers.WWWAuthenticate); got != "" {
			log.Debugf(ctx, "%s: %s", headers.WWWAuthenticate, got)
		}
		return nil, fmt.Errorf("fetch manifest for %s: http %s", image, resp.Status)
	}
	ct := resp.Header.Get(headers.ContentType)
	if mt, _, err := mime.ParseMediaType(ct); err != nil || mt != manifestMediaType {
		return nil, fmt.Errorf("fetch manifest for %s: unsupported %s %q", image, headers.ContentType, ct)
	}
	if resp.ContentLength == -1 || resp.ContentLength >= 10<<10 /* 10 MiB */ {
		return nil, fmt.Errorf("fetch manifest for %s: too large", image)
	}
	manifestJSON, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest for %s: %w", image, err)
	}
	parsed := new(imageManifest)
	if err := json.Unmarshal(manifestJSON, parsed); err != nil {
		return nil, fmt.Errorf("fetch manifest for %s: %w", image, err)
	}
	return parsed, nil
}

func downloadLayer(ctx context.Context, rootDir string, image, digest string, auth string) (string, error) {
	// See if we already downloaded the layer.
	dst := filepath.Join(rootDir, digest)
	if _, err := os.Stat(dst); err == nil {
		log.Debugf(ctx, "Layer %s already downloaded", digest)
		return dst, nil
	}

	// Download layer.
	mediaType, stream, err := downloadBlob(ctx, image, digest, auth)
	if err != nil {
		return "", fmt.Errorf("download layer: %w", err)
	}
	defer stream.Close()
	if mediaType != layerMediaType && mediaType != blobMediaType {
		return "", fmt.Errorf("download layer %s for %s: unsupported %s %q", digest, image, headers.ContentType, mediaType)
	}
	if err := os.MkdirAll(dst, 0777); err != nil {
		return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
	}
	zr, err := gzip.NewReader(stream)
	if err != nil {
		return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
	}
	r := tar.NewReader(zr)
	for {
		hdr, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
		}
		fname := filepath.Join(dst, filepath.FromSlash(hdr.Name))
		perm := hdr.FileInfo().Mode().Perm()
		switch hdr.Typeflag {
		case tar.TypeReg:
			f, err := os.Create(fname)
			if err != nil {
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			}
			if err := f.Chmod(perm); err != nil {
				f.Close()
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			}
			_, copyErr := io.Copy(f, r)
			closeErr := f.Close()
			if copyErr != nil {
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, copyErr)
			}
			if closeErr != nil {
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, closeErr)
			}
		case tar.TypeDir:
			if err := os.Mkdir(fname, perm); err != nil {
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(filepath.FromSlash(hdr.Linkname), fname); err != nil {
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			}
		case tar.TypeLink:
			linkname := filepath.Join(dst, hdr.Linkname)
			if err := os.Link(linkname, fname); err != nil {
				return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			}
		case tar.TypeChar:
			// dev := int(unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor)))
			// if err := unix.Mknod(fname, unix.S_IFCHR|uint32(perm), dev); err != nil {
			// 	return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			// }
			log.Debugf(ctx, "Attempted to create character device %d %d at %s", hdr.Devmajor, hdr.Devminor, hdr.Name)
		case tar.TypeBlock:
			// dev := int(unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor)))
			// if err := unix.Mknod(fname, unix.S_IFCHR|uint32(perm), dev); err != nil {
			// 	return "", fmt.Errorf("download layer %s for %s: %w", digest, image, err)
			// }
			log.Debugf(ctx, "Attempted to create block device %d %d at %s", hdr.Devmajor, hdr.Devminor, hdr.Name)
		default:
			return "", fmt.Errorf("download layer %s for %s: unhandled tar type %q", digest, image, hdr.Typeflag)
		}
	}
	return dst, nil
}

func downloadBlob(ctx context.Context, image, digest string, auth string) (mediaType string, _ io.ReadCloser, _ error) {
	name, _, _ := imageref.Parse(image)
	registry := imageref.Registry(name)
	name = strings.TrimPrefix(name, registry+"/")
	urlstr := "https://" + registry + "/v2/" + name + "/blobs/" + digest
	req, err := http.NewRequest(http.MethodGet, urlstr, nil)
	if err != nil {
		return "", nil, fmt.Errorf("fetch blob %s for %s: %w", digest, image, err)
	}
	req.Header.Set(headers.Authorization, auth)
	resp, err := doWithDockerAuth(ctx, http.DefaultClient, req, auth)
	if err != nil {
		return "", nil, fmt.Errorf("fetch blob %s for %s: %w", digest, image, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return "", nil, fmt.Errorf("fetch blob %s for %s: http %s", digest, image, resp.Status)
	}
	mediaType, _, err = mime.ParseMediaType(resp.Header.Get(headers.ContentType))
	if err != nil {
		resp.Body.Close()
		return "", nil, fmt.Errorf("fetch blob %s for %s: %s: %w", digest, image, headers.ContentType, err)
	}
	return mediaType, resp.Body, nil
}
