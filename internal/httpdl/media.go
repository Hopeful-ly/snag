package httpdl

import (
	"encoding/binary"
	"regexp"

	"github.com/Roham1299/snag/internal/watchlink"
)

var mediaName = regexp.MustCompile(`(?i)\.(mp4|m4v|mkv|webm|mov|avi|flv|ts|mpg|mpeg|wmv|ogv|3gp|mp3|m4a|ogg|opus|flac|wav|aac)$`)

func isMedia(path string) bool {
	return mediaName.MatchString(path)
}

// linkUp links the growing part file at the final name so it can be played mid-download.
func (d *download) linkUp() {
	if isMedia(d.path) {
		d.linked = watchlink.Make(d.path, d.partPath())
	}
}

const sniffSize = 64 << 10

// mp4Tail reads the start of the part file for mp4IndexOffset. The lock must be held.
func (d *download) mp4Tail() int64 {
	head := make([]byte, sniffSize)
	n, _ := d.file.ReadAt(head, 0)
	return mp4IndexOffset(head[:n])
}

var leadingBoxes = map[string]bool{"ftyp": true, "free": true, "skip": true, "wide": true, "pdin": true, "uuid": true, "styp": true}

// mp4IndexOffset returns the offset of a moov box placed after the media data, which players
// need before they can start. It returns 0 when moov is at the front or head isn't an MP4.
func mp4IndexOffset(head []byte) int64 {
	var at int64
	for at+8 <= int64(len(head)) {
		size := int64(binary.BigEndian.Uint32(head[at:]))
		kind := string(head[at+4 : at+8])
		if size == 1 && at+16 <= int64(len(head)) {
			size = int64(binary.BigEndian.Uint64(head[at+8:]))
		}
		switch {
		case kind == "moov":
			return 0
		case kind == "mdat":
			if size > 8 {
				return at + size
			}
			return 0
		case size < 8 || !leadingBoxes[kind]:
			return 0
		}
		at += size
	}
	return 0
}
