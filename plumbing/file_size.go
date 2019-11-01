package plumbing

import (
	"os"
)

// TODO switch to `yb config set binary-max-filesize=1MB`
const (
	BIN_MAX_FILESIZE = 1 * 1024 * 1024
)

func UnderMaxSize(filePath string) bool {

	if fi, err := os.Stat(filePath); err == nil && fi.Mode().IsRegular() {
		return fi.Size() <= BIN_MAX_FILESIZE
	}
	return false

}
