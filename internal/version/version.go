// Package version reports which build of Snag is running.
package version

// Version and Packaged are set at build time with -ldflags -X; Packaged is "true" for releases.
var (
	Version  = "dev"
	Packaged = ""
)

// IsPackaged reports whether this is an installed release rather than a development build.
func IsPackaged() bool {
	return Packaged == "true"
}

// String returns the version for display.
func String() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
