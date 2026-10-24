//go:build !linux

package platform

// BringForward does nothing here: a window's own request for focus is honored.
func BringForward(int, string) {}
