package buildpacks

import (
	"testing"
)

// Test_downloadURLVersion tests the different version formats for downloading
// flutter
func Test_downloadURLVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in:   "1.17.0",
			want: "1.17.0",
		},
		{
			in:   "1.18.0",
			want: "1.18.0",
		},
		{
			in:   "2.0.0",
			want: "2.0.0",
		},
		{
			in:   "v2.0.0",
			want: "2.0.0",
		},
		{
			in:   "v1.12.13+hotfix.8",
			want: "v1.12.13+hotfix.8",
		},
		{
			in:   "1.12.13+hotfix.8",
			want: "v1.12.13+hotfix.8",
		},
		{
			in:   "v1.12.0",
			want: "v1.12.0",
		},
		{
			in:   "1.12.0",
			want: "v1.12.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := downloadURLVersion(tt.in); got != tt.want {
				t.Errorf("downloadURLVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
