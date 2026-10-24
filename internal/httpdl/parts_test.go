package httpdl

import (
	"encoding/binary"
	"errors"
	"net/http"
	"testing"
)

func TestTakeHandsOutTheEarliestIdlePart(t *testing.T) {
	s := newSegments(10*inOrderChunk, true)
	first, second := s.take(true), s.take(true)
	if first.start != 0 || second.start != quickChunk || !first.busy {
		t.Errorf("took %d then %d", first.start, second.start)
	}
}

func TestAVideosStartComesInSmallPieces(t *testing.T) {
	s := newSegments(10*inOrderChunk, true)
	var sizes []int64
	for _, seg := range s.list {
		sizes = append(sizes, seg.end-seg.start+1)
	}
	small := quickStart / quickChunk
	for i, size := range sizes {
		want := int64(inOrderChunk)
		if i < small {
			want = quickChunk
		}
		if size != want {
			t.Fatalf("piece %d is %d bytes, want %d: %v", i, size, want, sizes)
		}
	}
}

func TestWantPutsAPlayersPositionFirst(t *testing.T) {
	s := newSegments(20*inOrderChunk, true)
	for range 4 {
		s.take(true) // connections busy on the start
	}
	far := int64(15*inOrderChunk + 1234)
	if half := s.want(far); half != nil {
		t.Errorf("split an idle part: %+v", half)
	}
	if got := s.take(true); got.start != 15*inOrderChunk {
		t.Errorf("next part from %d, want the one holding %d", got.start, far)
	}

	// The wanted byte is far ahead in a busy part: split there.
	whole := newSegments(40*minSplit, false)
	busy := whole.take(true)
	at := int64(30 * minSplit)
	half := whole.want(at)
	if half == nil || half.start != at || busy.end != at-1 || half.end != 40*minSplit-1 {
		t.Fatalf("split %+v from %+v", half, busy)
	}
	// Just ahead of the connection: no split.
	if again := whole.want(busy.pos + 1000); again != nil {
		t.Errorf("split just ahead of the connection: %+v", again)
	}
}

func TestTakePrefersTheMP4Index(t *testing.T) {
	s := newSegments(10*inOrderChunk, true)
	s.take(true)
	s.tailFrom = 9*inOrderChunk + 100
	if got := s.take(true); got.start != 9*inOrderChunk {
		t.Errorf("took %d, want the part holding the index", got.start)
	}
}

func TestTakeSplitsTheBiggestBusyPart(t *testing.T) {
	s := newSegments(8*minSplit, false)
	whole := s.take(true)
	half := s.take(true)
	if half == nil || half.start != 4*minSplit || whole.end != 4*minSplit-1 || half.end != 8*minSplit-1 {
		t.Fatalf("split into %+v and %+v", whole, half)
	}
	if s.take(false) != nil {
		t.Error("split a part when splitting isn't allowed")
	}
	small := newSegments(2*minSplit, false)
	small.take(true)
	if small.take(true) != nil {
		t.Error("split a part too small to share")
	}
}

func TestHaveCountsTheFinishedRunFromAnOffset(t *testing.T) {
	const c = inOrderChunk
	s := segments{wantFrom: -1}
	for at := int64(0); at < 4*c; at += c {
		s.list = append(s.list, &segment{start: at, pos: at, end: at + c - 1})
	}
	s.list[0].pos = c       // the first part is finished
	s.list[1].pos = c + 100 // the second has 100 bytes
	s.list[3].pos = 4 * c   // the last is finished, the third not begun
	for _, tc := range []struct{ off, want int64 }{
		{0, c + 100},   // through the first part into the second
		{50, c + 50},   // from inside the first part
		{c + 100, 0},   // where the second stops
		{2 * c, 0},     // the third part is empty
		{3 * c, c},     // the last part on its own
		{4*c - 10, 10}, // its last bytes
		{4 * c, 0},     // past the end
	} {
		if got := s.have(tc.off); got != tc.want {
			t.Errorf("have(%d) = %d, want %d", tc.off, got, tc.want)
		}
	}

	// A part split in two: the halves join up once the first reaches the second.
	split := newSegments(8*minSplit, false)
	whole := split.take(true)
	half := split.take(true)
	whole.pos = whole.end + 1
	half.pos = half.start + 10
	if got := split.have(0); got != half.start+10 {
		t.Errorf("across a split: have(0) = %d, want %d", got, half.start+10)
	}

	unknown := newSegments(0, false)
	if got := unknown.have(0); got != 0 {
		t.Errorf("unknown size, nothing written: have(0) = %d", got)
	}
}

func TestRemaining(t *testing.T) {
	s := newSegments(3*inOrderChunk, true)
	seg := s.take(true)
	seg.pos = seg.end + 1
	if got := s.remaining(); got != 3*inOrderChunk-quickChunk {
		t.Errorf("remaining = %d", got)
	}
}

func TestMP4IndexOffset(t *testing.T) {
	ftyp := box("ftyp", []byte("isom\x00\x00\x00\x00isommp41"))
	mdat := binary.BigEndian.AppendUint32(nil, 1000)
	mdat = append(mdat, "mdat"...)

	if got := mp4IndexOffset(append(ftyp, mdat...)); got != int64(len(ftyp))+1000 {
		t.Errorf("index after the media: %d, want %d", got, len(ftyp)+1000)
	}
	if got := mp4IndexOffset(append(ftyp, box("moov", nil)...)); got != 0 {
		t.Errorf("index at the front: %d, want 0", got)
	}
	if got := mp4IndexOffset([]byte("<html><body>not a video</body></html>")); got != 0 {
		t.Errorf("not an MP4: %d, want 0", got)
	}

	large := binary.BigEndian.AppendUint32(nil, 1)
	large = append(large, "mdat"...)
	large = binary.BigEndian.AppendUint64(large, 5<<32)
	if got := mp4IndexOffset(append(ftyp, large...)); got != int64(len(ftyp))+5<<32 {
		t.Errorf("64-bit mdat size: %d", got)
	}
}

func TestContentLength(t *testing.T) {
	tests := []struct {
		status       int
		contentRange string
		length       int64
		size         int64
		ranged       bool
	}{
		{206, "bytes 0-99/1234", 100, 1234, true},
		{206, "bytes 0-99/*", 100, 0, false},
		{200, "", 5000, 5000, false},
		{200, "", -1, 0, false},
	}
	for _, tt := range tests {
		resp := &http.Response{StatusCode: tt.status, Header: http.Header{}, ContentLength: tt.length}
		resp.Header.Set("Content-Range", tt.contentRange)
		size, ranged := contentLength(resp)
		if size != tt.size || ranged != tt.ranged {
			t.Errorf("%d %q: got %d, %v; want %d, %v", tt.status, tt.contentRange, size, ranged, tt.size, tt.ranged)
		}
	}
}

func TestIsPermanent(t *testing.T) {
	if !isPermanent(&StatusError{Code: 404}) || !isPermanent(errNoResume) || !isPermanent(&diskError{errors.New("full")}) {
		t.Error("a permanent error is retried")
	}
	if isPermanent(&StatusError{Code: 503}) || isPermanent(errClosedEarly) {
		t.Error("a passing error is not retried")
	}
	if got := (&StatusError{Code: 404}).Error(); got != "server said 404 Not Found" {
		t.Errorf("message %q", got)
	}
}
