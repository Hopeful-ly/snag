//go:build !linux

package platform

// InstallAppIcon does nothing: Windows and macOS take the icon from the binary.
func InstallAppIcon(_, _ []byte) {}
