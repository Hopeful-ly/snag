package platform

import "os/exec"

// PowerOff shuts the computer down.
func PowerOff() error {
	return run("systemctl", "poweroff")
}

// Sleep puts the computer to sleep.
func Sleep() error {
	return run("systemctl", "suspend")
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	hideConsole(cmd)
	return cmd.Run()
}
