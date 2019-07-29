// Package state provides shared state information for use by workers and the master.
package state

import "strings"

// relativePath takes the path to some file (original), and prepends that path
// (excluding the file at the end of the path) to another (other) path.
func relativePath(original, other string) string {
	return strings.Join([]string{strings.TrimRightFunc(original, func(ch rune) bool {return ch != '/' && ch != '\\'}), strings.TrimLeft(other, "/\\")}, "")
}