package platform

import (
	"errors"

	"golang.org/x/sys/unix"
)

// The attributes browsers, curl and wget record a download's origin in (freedesktop's common
// extended attributes), which file managers show.
const (
	originAttr   = "user.xdg.origin.url"
	referrerAttr = "user.xdg.referrer.url"
)

// MarkDownloaded records on a finished file where it came from, as browsers do. Linux doesn't
// check downloaded programs before they run, so that is all the mark does. A file system without
// user attributes can't be marked, which isn't an error.
func MarkDownloaded(path, source, page string) error {
	for _, attr := range []struct{ name, value string }{{originAttr, source}, {referrerAttr, page}} {
		value := webAddress(attr.value)
		if value == "" {
			continue
		}
		err := unix.Setxattr(path, attr.name, []byte(value), 0)
		if errors.Is(err, unix.ENOTSUP) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}
