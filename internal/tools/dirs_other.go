//go:build !darwin

package tools

// Elsewhere, user-installed programs are already on the PATH apps start with.
var wellKnownDirs []string
