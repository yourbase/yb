package workspace

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yourbase/yb/runtime"
)

func Test_parseEnvironment(t *testing.T) {
	type args struct {
		envPath     string
		runtimeData runtime.RuntimeEnvironmentData
		envPacks    [][]string
	}
	dummyRuntimeData := runtime.RuntimeEnvironmentData{Containers: runtime.ContainerData{}}
	egContents := []byte(`YB_PRECIOUS_SEKRET_KEY=something
THERE=no
YB_GITHUB_APP_ID=0000`)
	tempDir := t.TempDir()
	dotEnvFilePath := filepath.Join(tempDir, ".env")
	if err := ioutil.WriteFile(dotEnvFilePath, egContents, 0644); err != nil {
		t.Fatalf("Unable to write env file %s: %v", dotEnvFilePath, err)
	}
	t.Logf("Created %s for testing .env", dotEnvFilePath)

	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "No dotenv",
			args: args{
				envPath:     ".env",
				runtimeData: dummyRuntimeData,
				envPacks: [][]string{
					{
						"DATABASE_URL=test-ME",
						"YB_GITHUB_APP_ID=38644",
						"YB_APP_URL=http://localhost:3000",
					},
					{
						"AWS_SOMETHING_SOME=agoegoejo+Ej185",
					},
				},
			},
			want: []string{
				"AWS_SOMETHING_SOME=agoegoejo+Ej185",
				"DATABASE_URL=test-ME",
				"YB_APP_URL=http://localhost:3000",
				"YB_GITHUB_APP_ID=38644",
			},
		},
		{
			name: "With a dotenv",
			args: args{
				envPath:     dotEnvFilePath,
				runtimeData: dummyRuntimeData,
				envPacks: [][]string{
					{
						"DATABASE_URL=test-ME",
						"YB_GITHUB_APP_ID=38644",
						"YB_APP_URL=http://localhost:3000",
					},
					{
						"AWS_SOMETHING_SOME=agoegoejo+Ej185",
					},
				},
			},
			want: []string{
				"AWS_SOMETHING_SOME=agoegoejo+Ej185",
				"DATABASE_URL=test-ME",
				"THERE=no",
				"YB_APP_URL=http://localhost:3000",
				"YB_GITHUB_APP_ID=0000",
				"YB_PRECIOUS_SEKRET_KEY=something",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnvironment(context.Background(), tt.args.envPath, tt.args.runtimeData, tt.args.envPacks...)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want, cmpopts.SortSlices(func(i, j string) bool {
				return i < j
			})); diff != "" {
				t.Errorf("parseEnvironment(), diff: %v", diff)
			}
		})
	}
}
