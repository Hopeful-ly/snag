package platform

import "testing"

// useTestAutostartValue keeps the test's Run entry apart from a real Snag's, and removes it.
func useTestAutostartValue(t *testing.T) {
	old := autostartValue
	autostartValue = "SnagTest"
	t.Cleanup(func() {
		_ = Autostart{}.disable()
		autostartValue = old
	})
}
