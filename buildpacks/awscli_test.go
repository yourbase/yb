package buildpacks

import (
	"context"
	goruntime "runtime"
	"testing"

	"github.com/yourbase/yb/runtime"
)

func TestAWSCLIBuildTool_DownloadURL(t *testing.T) {
	type testCall struct {
		name    string
		version string
		URL     string
		wantErr bool
	}

	var tests []testCall

	if goruntime.GOOS == "linux" {
		tests = []testCall{
			{
				name:    "latest",
				version: "latest",
				URL:     "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip",
			},
			{
				name:    "version 2.0.49",
				version: "2.0.49",
				URL:     "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.0.49.zip",
			},
			{
				name:    "version 2.0.41",
				version: "2.0.41",
				URL:     "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.0.41.zip",
			},
			{
				name:    "version 2.0.30",
				version: "2.0.30",
				URL:     "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.0.30.zip",
			},
		}
	} else if goruntime.GOOS == "darwin" {
		tests = []testCall{
			{
				name:    "latest",
				version: "latest",
				URL:     "https://awscli.amazonaws.com/AWSCLIV2.pkg",
			},
			{
				name:    "version 2.0.49",
				version: "2.0.49",
				URL:     "https://awscli.amazonaws.com/AWSCLIV2-2.0.49.pkg",
			},
			{
				name:    "version 2.0.41",
				version: "2.0.41",
				URL:     "https://awscli.amazonaws.com/AWSCLIV2-2.0.41.pkg",
			},
			{
				name:    "version 2.0.30",
				version: "2.0.30",
				URL:     "https://awscli.amazonaws.com/AWSCLIV2-2.0.30.pkg",
			},
		}
	} else if goruntime.GOOS == "windows" || goruntime.GOARCH != "amd64" {
		tests = []testCall{
			{
				name:    "failed for unsupported Architecture",
				wantErr: true,
			},
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bt := NewAWSCLIBuildTool(BuildToolSpec{Tool: "awscli", Version: tt.version, PackageDir: "/opt/tools/awscli"})
			bt.spec.InstallTarget = &runtime.MetalTarget{}
			gotURL, err := bt.DownloadURL(context.Background())
			if !tt.wantErr && err != nil {
				t.Fatalf("Unable to generate download URL: %V", err)
			}
			if gotURL != tt.URL {
				t.Errorf("AWSCLIBuildTool.DownloadURL() = %v, want %v", gotURL, tt.URL)
			}
		})
	}
}
