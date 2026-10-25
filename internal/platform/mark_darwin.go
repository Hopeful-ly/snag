package platform

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// MarkDownloaded puts a finished file in quarantine, as browsers do: Gatekeeper then checks a
// program before it first runs, and Finder says the file was downloaded. The mark is the flags
// Safari sets on a download, when, and by whom; where from would go in a LaunchServices record,
// which needs its framework, so the record's id is left empty. A file system without extended
// attributes can't be marked, which isn't an error.
func MarkDownloaded(path, _, _ string) error {
	value := fmt.Sprintf("0081;%x;Snag;", time.Now().Unix())
	err := unix.Setxattr(path, "com.apple.quarantine", []byte(value), 0)
	if errors.Is(err, unix.ENOTSUP) {
		return nil
	}
	return err
}
