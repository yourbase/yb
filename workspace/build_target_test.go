package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/yourbase/yb/runtime"
)

func TestBuildTarget_EnvironmentVariables(t *testing.T) {
	type fields struct {
		Name        string
		Tools       []string
		Commands    []string
		Environment []string
	}
	type args struct {
		data runtime.RuntimeEnvironmentData
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
				Name: "default",
				Commands: []string{
					"pip install -r requirements.txt",
					"pylint .",
				},
				Environment: []string{
					"DATABASE_URL=testdb",
					"YB_GITHUB_APP_ID=38644",
					"YB_APP_URL=http://localhost:3000",
				},
			},
			args: args{
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
			bt := BuildTarget{
				Name:        tt.fields.Name,
				Tools:       tt.fields.Tools,
				Commands:    tt.fields.Commands,
				Environment: tt.fields.Environment,
			}
			sort.Slice(tt.want, func(i, j int) bool { return tt.want[i] < tt.want[j] })
			got := bt.EnvironmentVariables(tt.args.data)
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildTarget.EnvironmentVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}
