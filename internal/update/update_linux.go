package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// The packages put Snag here, and link /usr/bin/snag to it.
const (
	packaged = "/usr/lib/snag/snag"
	launcher = "/usr/bin/snag"
)

var kind = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil || exe != packaged {
		return ""
	}
	for _, m := range managers {
		if exec.Command(m.owns[0], append(m.owns[1:], exe)...).Run() == nil {
			return m.format + "-" + runtime.GOARCH
		}
	}
	return ""
})

// managers are the package managers Snag's packages are for, and how each tells it owns a file.
var managers = []struct {
	format string
	owns   []string
}{
	{"arch", []string{"pacman", "-Qo"}},
	{"deb", []string{"dpkg-query", "-S"}},
	{"rpm", []string{"rpm", "-qf"}},
}

// installCommand is the command that installs the package at file, as root.
func installCommand(format, file string) []string {
	has := func(program string) bool { _, err := exec.LookPath(program); return err == nil }
	switch format {
	case "arch":
		return []string{"pacman", "-U", "--noconfirm", file}
	case "deb":
		if has("apt-get") {
			return []string{"apt-get", "install", "-y", "--allow-downgrades", file}
		}
		return []string{"dpkg", "-i", file}
	case "rpm":
		if has("dnf") {
			return []string{"dnf", "install", "-y", file}
		}
		if has("zypper") {
			return []string{"zypper", "--non-interactive", "install", "--allow-unsigned-rpm", file}
		}
		return []string{"rpm", "-U", file}
	}
	return nil
}

// install asks for the administrator password (pkexec) to install the package, then starts the
// new Snag once this one has quit.
func install(file string) error {
	format, _, _ := strings.Cut(kind(), "-")
	command := installCommand(format, file)
	if command == nil {
		return errors.New("this copy of Snag can't update itself")
	}
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return fmt.Errorf("pkexec, to ask for the password, isn't installed: install %s yourself", file)
	}
	out, err := exec.Command(pkexec, command...).CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) {
		return errors.New("the update wasn't allowed")
	}
	if err != nil {
		return fmt.Errorf("installing the update failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return afterExit(`exec "$1"`, launcher)
}
