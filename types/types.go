package types

import (
	"context"
)

const (
	MANIFEST_FILE        = ".yourbase.yml"
	DOCS_URL             = "https://docs.yourbase.io"
	DEFAULT_YB_CONTAINER = "yourbase/yb_ubuntu:18.04"
)

type EnvVariable struct {
	Key   string
	Value string
}

// API Responses -- TODO use Swagger instead, this is silly
type Project struct {
	Id          int    `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	OrgSlug     string `json:"organization_slug"`
}

type TokenResponse struct {
	TokenOK bool `json:"token_ok"`
}

type WorktreeSave struct {
	Hash    string
	Path    string
	Files   []string
	Enabled bool
}

type BuildTool interface {
	Install(ctx context.Context) (string, error)
	Setup(ctx context.Context, dir string) error
	DownloadURL(ctx context.Context) (string, error)
	Version() string
}
