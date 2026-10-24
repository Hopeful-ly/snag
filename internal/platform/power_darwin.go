package platform

import "os/exec"

// PowerOff shuts down as the Apple menu does, letting apps save first.
func PowerOff() error {
	return run("osascript", "-e", `tell application "System Events" to shut down`)
}

// Sleep puts the computer to sleep.
func Sleep() error {
	return run("pmset", "sleepnow")
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	hideConsole(cmd)
	return cmd.Run()
}
