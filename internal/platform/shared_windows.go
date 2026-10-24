package platform

import (
	"os"
	"syscall"
)

// OpenShared opens a file for reading while letting others rename or delete it, which Windows otherwise refuses.
func OpenShared(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	const share = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ, share, nil, syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
