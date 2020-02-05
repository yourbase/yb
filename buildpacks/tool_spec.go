package buildpacks

import (
	"bytes"
	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
	"text/template"
)

type BuildToolSpec struct {
	Tool            string
	Version         string
	SharedCacheDir  string
	PackageCacheDir string
	PackageDir      string
	InstallTarget   runtime.Target
}

func TemplateToString(templateText string, data interface{}) (string, error) {
	t, err := template.New("generic").Parse(templateText)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		log.Errorf("Can't render template:: %v", err)
		return "", err
	}

	result := tpl.String()
	return result, nil
}