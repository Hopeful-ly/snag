//go:build !windows

package platform

import "os"

// OpenShared opens a file for reading; Unix already allows renaming or deleting it meanwhile.
func OpenShared(path string) (*os.File, error) { return os.Open(path) }
