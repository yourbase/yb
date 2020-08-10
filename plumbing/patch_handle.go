package plumbing

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/beholders-eye/diffparser"
	"github.com/yourbase/yb/plumbing/log"
)

func applyPatch(file *diffparser.DiffFile, from string) (to string, err error) {
	if file == nil {
		return "", fmt.Errorf("Needs a file to process")
	}
	lines := strings.Split(from, "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("Won't process an empty string")
	}
	var unmatchedLines int64
	for _, hunk := range file.Hunks {
		idx := hunk.OrigRange.Start - 1
		if idx < 0 {
			return "", fmt.Errorf("Malformed patch, wrong start position (%d): %v\n", idx, strings.Join(file.Changes, "\n"))
		}
		for i, line := range hunk.OrigRange.Lines {
			index := idx + i
			passedLine := lines[index]
			if !strings.EqualFold(line.Content, passedLine) {
				unmatchedLines++
			}
		}

		before := lines[:idx]
		after := lines[idx+hunk.OrigRange.Length:]
		var result []string

		for _, line := range hunk.WholeRange.Lines {
			if line.Mode != diffparser.REMOVED {
				result = append(result, line.Content)
			}
		}
		newLines := append(before, result...)
		newLines = append(newLines, after...)
		lines = newLines
	}
	if unmatchedLines > 0 {
		log.Debugf("%d unmatched lines in this file\n", unmatchedLines)
	}
	to = strings.Join(lines, "\n")

	return
}

// TODO use to replace calling the `patch -p1 -i` commmand on the Build Agent
// UnifiedPatchApply apply git formated patches to a local directory or and git.Worktree
//   If the worktree isn't passsed it will try working on the local directory
func UnifiedPatchOnGit(patch string, commit *object.Commit, w, originWorktree *git.Worktree, wd string) (patchError error) {
	if commit == nil && w == nil && wd == "" {
		return fmt.Errorf("Nowhere to apply the patch on, please pass a git commit + git worktree, or a workdir to work on files")
	}

	getCommitFileContents := func(file *diffparser.DiffFile) (contents string) {
		tree, err := commit.Tree()
		if err != nil {
			patchError = fmt.Errorf("Unable to resolve commit tree: %v", err)
			return ""
		}
		var workFile *object.File
		switch file.Mode {
		case diffparser.DELETED:
			return ""
		case diffparser.MODIFIED:
			workFile, err = tree.File(file.OrigName)
			if err != nil {
				patchError = fmt.Errorf("Unable to retrieve tree entry %s: %v", file.OrigName, err)
				return ""
			}
			contents, err = workFile.Contents()
		case diffparser.NEW:
			newFile, err := originWorktree.Filesystem.Open(file.NewName)
			if err != nil {
				patchError = fmt.Errorf("Unable to open %s on the work tree: %v", file.NewName, err)
				return ""
			}
			newBytes := bytes.NewBuffer(nil)
			_, err = io.Copy(newBytes, newFile)
			if err != nil {
				patchError = fmt.Errorf("Unable to copy %s to a buffer: %v", file.NewName, err)
				return ""
			}
			contents = newBytes.String()
			_ = newFile.Close()
		}
		if err != nil {
			patchError = fmt.Errorf("Couldn't get contents of %s: %v", file.OrigName, err)
			return ""
		}
		return
	}

	// Detect in the patch string (unified) which files were affected and how
	diff, err := diffparser.Parse(patch)
	if err != nil {
		return fmt.Errorf("Generated patch parse failed: %v", err)
	}

	for _, file := range diff.Files {
		//TODO move files (should be implemented in github.com/beholders-eye/diffparser)
		switch file.Mode {
		case diffparser.DELETED:
			w.Remove(file.OrigName)
		case diffparser.MOVED:
			return fmt.Errorf("Not implemented yet")
		default:
			contents := getCommitFileContents(file)
			if contents == "" && patchError != nil {
				return fmt.Errorf("Unable to fetch contents from %s: %v", file, patchError)
			}

			var fixedContents string
			if file.Mode == diffparser.MODIFIED {
				fixedContents, err = applyPatch(file, contents)
				if err != nil {
					return fmt.Errorf("Apply Patch failed for <%s>: %v", file, err)
				}
			}

			if w != nil {
				newFile, err := w.Filesystem.Create(file.NewName)
				if err != nil {
					return fmt.Errorf("Unable to open %s on the cloned tree: %v", file.NewName, err)
				}

				var c string
				if file.Mode == diffparser.MODIFIED {
					c = fixedContents
				} else {
					c = contents
				}
				if _, err = newFile.Write([]byte(c)); err != nil {
					return fmt.Errorf("Unable to write patch hunk to %s: %v", file.NewName, err)
				}
				_ = newFile.Close()

				w.Add(file.NewName)
			} else if wd != "" {
				//TODO Change file contents directly on the directory
				return fmt.Errorf("Not implemented")
			}
		}
	}
	return
}
