package subs

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Roham1299/snag/internal/clip"
)

var srtTimes = regexp.MustCompile(`^\s*(\d+):(\d\d):(\d\d)[,.](\d{1,3})\s*-->\s*(\d+):(\d\d):(\d\d)[,.](\d{1,3})`)

// cutSRT keeps the cues shown during r, retimed relative to r.From.
func cutSRT(srt []byte, r clip.Range) []byte {
	from, to := r.From, r.To
	text := strings.ReplaceAll(string(bytes.TrimPrefix(srt, []byte{0xEF, 0xBB, 0xBF})), "\r\n", "\n")
	var out strings.Builder
	n := 0
	for _, block := range strings.Split(text, "\n\n") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		stamp := -1
		for i, line := range lines {
			if strings.Contains(line, "-->") {
				stamp = i
				break
			}
		}
		if stamp < 0 {
			continue
		}
		m := srtTimes.FindStringSubmatch(lines[stamp])
		if m == nil {
			continue
		}
		start, end := srtSeconds(m[1:5]), srtSeconds(m[5:9])
		if end <= from || start >= to {
			continue
		}
		n++
		fmt.Fprintf(&out, "%d\n%s --> %s\n%s\n\n", n,
			srtStamp(max(start, from)-from), srtStamp(min(end, to)-from), strings.Join(lines[stamp+1:], "\n"))
	}
	return []byte(out.String())
}

func srtSeconds(m []string) float64 {
	h, _ := strconv.Atoi(m[0])
	mins, _ := strconv.Atoi(m[1])
	s, _ := strconv.Atoi(m[2])
	ms, _ := strconv.Atoi((m[3] + "00")[:3])
	return float64(h*3600+mins*60+s) + float64(ms)/1000
}

func srtStamp(seconds float64) string {
	ms := int(seconds*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
