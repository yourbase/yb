package types

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnewart/archiver"
)

type WorktreeSave struct {
	Hash    string
	Path    string
	Files   []string
	Enabled bool
}

func pathExists(path string) bool {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return false
	}

	return true
}

func (w *WorktreeSave) Add(file string) (err error) {
	if w.Enabled {
		fullPath := filepath.Join(w.Path, file)
		if !pathExists(fullPath) {
			err = fmt.Errorf("Path %v, for saving worktree state, doesn't exist", fullPath)
			return
		}
		w.Files = append(w.Files, file)
	}
	return
}

func (w *WorktreeSave) Save() (worktreeSaveFile string, err error) {
	if w.Enabled {
		if len(w.Files) > 0 && len(w.Hash) > 0 {
			// Save a tarball
			tar := archiver.Tar{
				MkdirAll: true,
			}
			worktreeSaveFile = filepath.Join(w.Path, fmt.Sprintf(".yb-worktreesave-%s.tar", w.Hash))
			err = tar.Archive(w.Files, worktreeSaveFile)
		} else {
			err = fmt.Errorf("Need files and a commit hash")
		}
	}
	return
}

func (w *WorktreeSave) Restore(pkgFile string) (err error) {
	if w.Enabled {
		if !pathExists(pkgFile) || !pathExists(w.Path) {
			err = fmt.Errorf("Path doesn't exist: %v or %v", pkgFile, w.Path)
			return
		}

		tar := archiver.Tar{OverwriteExisting: true}
		err = tar.Unarchive(pkgFile, w.Path)
	}
	return
}

func NewWorktreeSave(path string, hash string, enabled bool) (w *WorktreeSave, err error) {
	if enabled {
		if !pathExists(path) {
			err = fmt.Errorf("Path '%v', for saving worktree state, doesn't exist", path)
			return
		}

		if len(hash) == 0 {
			err = fmt.Errorf("Need a commit hash")
			return
		}
	}
	w = &WorktreeSave{Path: path, Hash: hash, Enabled: enabled}
	return
}
