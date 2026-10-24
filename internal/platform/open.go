package platform

import "os/exec"

// launch starts a program without waiting for it.
func launch(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
