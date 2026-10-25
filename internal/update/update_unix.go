//go:build linux || darwin

package update

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/Roham1299/snag/internal/platform"
)

// afterExit runs script in sh once this process has exited, with args as $1, $2 and so on.
func afterExit(script string, args ...string) error {
	wait := `while kill -0 "$0" 2>/dev/null; do sleep 0.2; done; `
	cmd := exec.Command("/bin/sh", append([]string{"-c", wait + script, strconv.Itoa(os.Getpid())}, args...)...)
	platform.StartDetached(cmd)
	return cmd.Start()
}
