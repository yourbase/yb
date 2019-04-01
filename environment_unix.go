// +build !windows

package main

import (
	"fmt"
	"strings"
)

func BuildPathString(paths []string) string {
	return strings.Join(paths, ":")
}

func PrependPath(newElement string, existing string) string {
	return fmt.Sprintf("%s:%s", newElement, existing)
}

func AppendPath(newElement string, existing string) string {
	return fmt.Sprintf("%s:%s", existing, newElement)
}
