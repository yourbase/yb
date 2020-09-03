package workspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb/runtime"
)

func TestExecPhase_EnvironmentVariablesDotEnv(t *testing.T) {
	type fields struct {
		Name         string
		Dependencies ExecDependencies
		Container    narwhal.ContainerDefinition
		Commands     []string
		Ports        []string
		Environment  map[string][]string
		HostOnly     bool
	}
	type args struct {
		ctx     context.Context
		envName string
		data    runtime.RuntimeEnvironmentData
	}
	// Sets up dotenv
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Unable to find current work dir: %v", err)
	}
	testPath := filepath.Join(cwd, "TESTING")
	err = os.Mkdir(testPath, 0755)
	if err != nil {
		t.Fatalf("Unable to make test dir: %v", err)
	}
	dotEnvFilePath := filepath.Join(testPath, ".env")
	err = os.Chdir(testPath)
	if err != nil {
		t.Fatalf("Unable to change into test dir: %v", err)
	}

	if err = ioutil.WriteFile(dotEnvFilePath,
		[]byte(`YB_PRECIOUS_SEKRET_KEY=something
THERE=no
YB_GITHUB_APP_ID=0000`),
		0644); err != nil {

		t.Fatalf("Unable to write env file %s: %v", dotEnvFilePath, err)
	}
	defer func() {
		err = os.Remove(dotEnvFilePath)
		if err != nil {
			t.Errorf("Unable to delete %s: %v", dotEnvFilePath, err)
		}
		err = os.Chdir(cwd)
		if err != nil {
			t.Errorf("Unable to change into old CWD %s: %v", cwd, err)
		}
		err = os.Remove(testPath)
		if err != nil {
			t.Errorf("Unable to remote dir %s: %v", testPath, err)
		}
	}()

	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{
			name: "basic",
			fields: fields{
				Name: "api",
				Commands: []string{
					"pip install -r requirements.txt",
					"black .",
					"bandit .",
					"honcho start -f Procfile.dev",
				},
				Ports: []string{
					"5000",
				},
				Environment: map[string][]string{
					"default": {
						"DATABASE_URL=testdb",
						"YB_GITHUB_APP_ID=38644",
						"YB_APP_URL=http://localhost:3000",
					},
				},
				HostOnly: false,
			},
			args: args{
				ctx:     context.Background(),
				envName: "default",
				data: runtime.RuntimeEnvironmentData{
					Containers: runtime.ContainerData{},
				},
			},
			want: []string{
				"DATABASE_URL=testdb",
				"YB_GITHUB_APP_ID=0000",
				"YB_APP_URL=http://localhost:3000",
				"YB_PRECIOUS_SEKRET_KEY=something",
				"THERE=no",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ExecPhase{
				Name:         tt.fields.Name,
				Dependencies: tt.fields.Dependencies,
				Container:    tt.fields.Container,
				Commands:     tt.fields.Commands,
				Ports:        tt.fields.Ports,
				Environment:  tt.fields.Environment,
				HostOnly:     tt.fields.HostOnly,
			}
			sort.Slice(tt.want, func(i, j int) bool { return tt.want[i] < tt.want[j] })
			got := e.EnvironmentVariables(tt.args.ctx, tt.args.envName, tt.args.data)
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExecPhase.EnvironmentVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}
