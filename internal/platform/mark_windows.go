package platform

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// MarkDownloaded gives a finished file the Mark of the Web, as browsers do: a Zone.Identifier
// stream saying it came from the internet (zone 3), and from where. SmartScreen then checks a
// program before it first runs, and Office opens a document in Protected View. A drive whose
// file system has no streams (FAT, exFAT) can't be marked, which isn't an error.
func MarkDownloaded(path, source, page string) error {
	var b strings.Builder
	b.WriteString("[ZoneTransfer]\r\nZoneId=3\r\n")
	if page := webAddress(page); page != "" {
		fmt.Fprintf(&b, "ReferrerUrl=%s\r\n", page)
	}
	if source := webAddress(source); source != "" {
		fmt.Fprintf(&b, "HostUrl=%s\r\n", source)
	}
	err := os.WriteFile(path+":Zone.Identifier", []byte(b.String()), 0o644)
	if errors.Is(err, windows.ERROR_INVALID_NAME) {
		return nil
	}
	return err
}
