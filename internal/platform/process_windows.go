package platform

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// StartGroup makes cmd start without a console window, in a process group of its own.
func StartGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

// StopGroup kills cmd and its children; without a console, Windows can't send them Ctrl-C.
func StopGroup(cmd *exec.Cmd, _ bool) error {
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return kill.Run()
}

// hideConsole suppresses cmd's console window but not its own windows.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

const detachedProcess = 0x00000008 // DETACHED_PROCESS

// StartDetached makes cmd start detached, so it outlives its parent.
func StartDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
