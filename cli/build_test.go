package cli

import (
	"testing"
)

func Test_parseArg(t *testing.T) {
	tests := []struct {
		in     string
		target string
		pkg    string
	}{
		{
			in:     "default",
			target: "default",
		},
		{
			in:     "@local:test",
			target: "test",
			pkg:    "local",
		},
		{
			in:     "@api:linting",
			target: "linting",
			pkg:    "api",
		},
	}
	for _, tt := range tests {
		pkg, target, err := parseArgs(tt.in)
		if err != nil {
			t.Fatalf("parseArgs() returned an error: %v", err)
		}
		if target != tt.target {
			t.Errorf("parseArgs target: %s, want: %s", target, tt.target)
		}
		if pkg != tt.pkg {
			t.Errorf("parseArgs pkg: %s, want: %s", pkg, tt.pkg)
		}
	}
}
