package nativemsg

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	maxFromBrowser = 8 << 20
	maxToBrowser   = 1 << 20 // browsers refuse larger messages from a helper
)

var errTooLarge = errors.New("message too large")

// readFrame reads one frame: a length in native byte order, then JSON. An oversized frame is
// skipped and reported as errTooLarge.
func readFrame(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.NativeEndian, &length); err != nil {
		return nil, err
	}
	if length > maxFromBrowser {
		_, _ = io.CopyN(io.Discard, r, int64(length))
		return nil, errTooLarge
	}
	message := make([]byte, length)
	if _, err := io.ReadFull(r, message); err != nil {
		return nil, err
	}
	return message, nil
}

func writeFrame(w io.Writer, message []byte) error {
	if len(message) > maxToBrowser {
		return errTooLarge
	}
	frame := binary.NativeEndian.AppendUint32(make([]byte, 0, 4+len(message)), uint32(len(message)))
	_, err := w.Write(append(frame, message...))
	return err
}
