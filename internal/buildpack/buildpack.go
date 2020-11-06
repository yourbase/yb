package buildpack

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/ybdata"
	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"zombiezen.com/go/log"
)

// Sys holds dependencies provided by the caller needed to run builds.
type Sys struct {
	Biome    biome.Biome
	Stdout   io.Writer
	Stderr   io.Writer
	DataDirs *ybdata.Dirs

	HTTPClient *http.Client

	DockerClient    *docker.Client
	DockerNetworkID string
}

var packs = map[string]func(context.Context, Sys, yb.BuildpackSpec) (biome.Environment, error){
	"anaconda2":  installAnaconda2,
	"anaconda3":  installAnaconda3,
	"android":    installAndroidSDK,
	"androidndk": installAndroidNDK,
	"ant":        installAnt,
	"dart":       installDart,
	"flutter":    installFlutter,
	"glide":      installGlide,
	"go":         installGo,
	"gradle":     installGradle,
	"heroku":     installHeroku,
	"java":       installJava,
	"maven":      installMaven,
	"node":       installNode,
	"protoc":     installProtoc,
	"python":     installPython,
	"r":          installR,
	"ruby":       installRuby,
	"rust":       installRust,
	"yarn":       installYarn,
}

type buildToolSpec struct {
	tool       yb.BuildpackSpec
	dataDirs   *ybdata.Dirs
	cacheDir   string
	packageDir string
}

// Install installs the buildpack given by spec into the biome.
func Install(ctx context.Context, sys Sys, spec yb.BuildpackSpec) (_ biome.Environment, err error) {
	ctx, span := ybtrace.Start(ctx, "Buildpack "+string(spec), trace.WithAttributes(
		label.String("buildpack", spec.Name()),
		label.String("spec", string(spec)),
	))
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()
	log.Infof(ctx, "Configuring build tool %s...", spec)
	f := packs[spec.Name()]
	if f == nil {
		return biome.Environment{}, fmt.Errorf("install buildpack %s: no such buildpack", spec)
	}
	env, err := f(ctx, sys, spec)
	if err != nil {
		return biome.Environment{}, fmt.Errorf("install buildpack %s: %w", spec, err)
	}
	log.Debugf(ctx, "Build tool %s environment:\n%v", spec, env)
	return env, nil
}

// Extract modes.
const (
	// Archive does not contain a top-level directory.
	tarbomb = false
	// Remove the archive's top-level directory.
	stripTopDirectory = true
)

// extract downloads the given URL and extracts it to the given directory in the biome.
func extract(ctx context.Context, sys Sys, dstDir, url string, extractMode bool) (err error) {
	const (
		zipExt    = ".zip"
		tarXZExt  = ".tar.xz"
		tarGZExt  = ".tar.gz"
		tarBZ2Ext = ".tar.bz2"
	)
	exts := []string{
		zipExt,
		tarXZExt,
		tarGZExt,
		tarBZ2Ext,
	}
	var ext string
	for _, testExt := range exts {
		if strings.HasSuffix(url, testExt) {
			ext = testExt
			break
		}
	}
	if ext == "" {
		return fmt.Errorf("extract %s in %s: unknown extension", url, dstDir)
	}

	f, err := ybdata.Download(ctx, sys.HTTPClient, sys.DataDirs, url)
	if err != nil {
		return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
	}
	defer f.Close()

	defer func() {
		// Attempt to clean up if unarchive fails.
		if err != nil {
			rmErr := sys.Biome.Run(ctx, &biome.Invocation{
				Argv:   []string{"rm", "-rf", dstDir},
				Stdout: sys.Stdout,
				Stderr: sys.Stderr,
			})
			if rmErr != nil {
				log.Warnf(ctx, "Failed to clean up %s: %v", dstDir, rmErr)
			}
		}
	}()
	err = biome.MkdirAll(ctx, sys.Biome, dstDir)
	if err != nil {
		return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
	}
	dstFile := dstDir + ext
	defer func() {
		rmErr := sys.Biome.Run(ctx, &biome.Invocation{
			Argv:   []string{"rm", "-f", dstFile},
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if rmErr != nil {
			log.Warnf(ctx, "Failed to clean up %s: %v", dstFile, rmErr)
		}
	}()
	err = biome.WriteFile(ctx, sys.Biome, dstFile, f)
	if err != nil {
		return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
	}

	invoke := &biome.Invocation{
		Dir:    biome.AbsPath(sys.Biome, dstDir),
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	}
	absDstFile := biome.AbsPath(sys.Biome, dstFile)
	switch ext {
	case zipExt:
		invoke.Argv = []string{"unzip", "-q", absDstFile}
	case tarXZExt:
		invoke.Argv = []string{
			"tar",
			"-x", // extract
			"-J", // xz
			"-f", absDstFile,
		}
		if extractMode == stripTopDirectory {
			invoke.Argv = append(invoke.Argv, "--strip-components", "1")
		}
	case tarGZExt:
		invoke.Argv = []string{
			"tar",
			"-x", // extract
			"-z", // gzip
			"-f", absDstFile,
		}
		if extractMode == stripTopDirectory {
			invoke.Argv = append(invoke.Argv, "--strip-components", "1")
		}
	case tarBZ2Ext:
		invoke.Argv = []string{
			"tar",
			"-x", // extract
			"-j", // bzip2
			"-f", absDstFile,
		}
		if extractMode == stripTopDirectory {
			invoke.Argv = append(invoke.Argv, "--strip-components", "1")
		}
	default:
		panic("unreachable")
	}
	if err := sys.Biome.Run(ctx, invoke); err != nil {
		return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
	}
	if ext == zipExt && extractMode == stripTopDirectory {
		// There's no convenient way of stripping the top-level directory from an
		// unzip invocation, but we can move the files ourselves.
		size, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("extract %s in %s: determine archive size: %w", url, dstDir, err)
		}
		zr, err := zip.NewReader(f, size)
		if err != nil {
			return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
		}
		root, names, err := topLevelZipFilenames(zr.File)
		if err != nil {
			return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
		}

		mvArgv := []string{"mv"}
		for _, name := range names {
			mvArgv = append(mvArgv, sys.Biome.JoinPath(root, name))
		}
		mvArgv = append(mvArgv, ".")
		err = sys.Biome.Run(ctx, &biome.Invocation{
			Argv:   mvArgv,
			Dir:    biome.AbsPath(sys.Biome, dstDir),
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if err != nil {
			return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
		}
		err = sys.Biome.Run(ctx, &biome.Invocation{
			Argv:   []string{"rmdir", root},
			Dir:    biome.AbsPath(sys.Biome, dstDir),
			Stdout: sys.Stdout,
			Stderr: sys.Stderr,
		})
		if err != nil {
			return fmt.Errorf("extract %s in %s: %w", url, dstDir, err)
		}
	}
	return nil
}

// topLevelZipFilenames returns the names of the direct children of the root zip
// file directory.
func topLevelZipFilenames(files []*zip.File) (root string, names []string, _ error) {
	if len(files) == 0 {
		return "", nil, nil
	}
	i := strings.IndexByte(files[0].Name, '/')
	if i == -1 {
		return "", nil, fmt.Errorf("find zip root directory: %q not in a directory", files[0].Name)
	}
	root = files[0].Name[:i]
	prefix := files[0].Name[:i+1]
	for _, f := range files {
		if !strings.HasPrefix(f.Name, prefix) {
			return "", nil, fmt.Errorf("find zip root directory: %q not in directory %q", f.Name, root)
		}
		name := f.Name[i+1:]
		if nameEnd := strings.IndexByte(name, '/'); nameEnd != -1 {
			name = name[:nameEnd]
		}
		if name == root {
			return "", nil, fmt.Errorf("strip zip root directory: %q contains a file %q", name, name)
		}
		if name != "" && !stringInSlice(names, name) {
			names = append(names, name)
		}
	}
	return
}

func templateToString(templateText string, data interface{}) (string, error) {
	t, err := template.New("generic").Parse(templateText)
	if err != nil {
		return "", err
	}
	expanded := new(strings.Builder)
	if err := t.Execute(expanded, data); err != nil {
		return "", err
	}
	return expanded.String(), nil
}

func stringInSlice(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}
