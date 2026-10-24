//go:build !windows

package platform

import (
	"os/exec"
	"syscall"
)

// StartGroup makes cmd start in a process group of its own, so StopGroup also reaches the
// programs it starts (yt-dlp runs ffmpeg).
func StartGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// StopGroup sends the group SIGINT so programs can save state, or kills it with force.
func StopGroup(cmd *exec.Cmd, force bool) error {
	signal := syscall.SIGINT
	if force {
		signal = syscall.SIGKILL
	}
	return syscall.Kill(-cmd.Process.Pid, signal)
}

// hideConsole is a no-op outside Windows.
func hideConsole(*exec.Cmd) {}

// StartDetached makes cmd start in its own session, so it outlives its parent.
func StartDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
