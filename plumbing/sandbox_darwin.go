package plumbing

import (
  "fmt"
  "io/ioutil"
  "os"
)

const SANDBOX_TEMPLATE = `
(version 1) 
(deny default)

;; Only allow write in our workspace
(allow file-write* file-read-data file-read-metadata
  (regex "^{{.WorkspacePath}}/.*"))

;; You can also add a sperate section for reading and writing files outside your
;; user_name account directory.
(allow file-read-data file-read-metadata
  (regex "^{{.ToolsDir}}/.*"))

;; If you want to import extra rules from 
;; an existing sandbox configuration file: 
;;(import "/usr/share/sandbox/bsd.sb")

;; If your MyApp wants to run extra processes it's be allowed to run only
;; child processes and nothign else
(allow process-exec*)

;; If your MyApp requires network access you can grant it here:
(allow network*)
`
type SandboxParameters struct {
  WorkspacePath        string
  ToolsDir      string
}

func ExecInSandbox(command string, workingDir string) error {		
  workspace := LoadWorkspace()
  sandboxFile, err := ioutil.TempFile("", "sandbox-*") 
  defer os.Remove(sandboxFile.Name())

  sandboxParams := SandboxParameters{
                         WorkspacePath: workspace.Path, 
                         ToolsDir: ToolsDir(),
                   }

  sandboxContents, err := TemplateToString(SANDBOX_TEMPLATE, sandboxParams) 
  if err != nil { 
    return err
  }

  _, err = sandboxFile.WriteString(sandboxContents)
  if err != nil {
    return err 
  }
  sandboxFile.Close()
  sandboxedCommand := fmt.Sprintf("sandbox-exec -f %s %s", sandboxFile.Name(), command)
  return ExecToStdout(sandboxedCommand, workingDir)
}
