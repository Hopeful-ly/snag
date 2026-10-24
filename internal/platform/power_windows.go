package platform

import (
	"os/exec"
	"syscall"
)

// PowerOff shuts the computer down.
func PowerOff() error {
	cmd := exec.Command("shutdown", "/s", "/t", "0")
	hideConsole(cmd)
	return cmd.Run()
}

var setSuspendState = syscall.NewLazyDLL("powrprof.dll").NewProc("SetSuspendState")

// Sleep puts the computer to sleep (or hibernates it, where Windows is set to instead).
func Sleep() error {
	if err := setSuspendState.Find(); err != nil {
		return err
	}
	if ok, _, err := setSuspendState.Call(0, 0, 0); ok == 0 {
		return err
	}
	return nil
}
