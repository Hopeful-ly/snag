package subs

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

func vttToSRT(src []byte) ([]byte, error) {
	s := string(bytes.TrimPrefix(src, []byte{0xEF, 0xBB, 0xBF}))
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	if !strings.HasPrefix(s, "WEBVTT") && !strings.Contains(s, ".") {
		return nil, errors.New("not WebVTT")
	}
	body := s
	if i := strings.Index(s, "\n\n"); i >= 0 && strings.HasPrefix(s, "WEBVTT") {
		body = s[i+2:]
	} else if strings.HasPrefix(s, "WEBVTT") {
		if j := strings.IndexByte(s, '\n'); j >= 0 {
			body = s[j+1:]
		}
	}
	var cues []string
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "NOTE") || strings.HasPrefix(block, "STYLE") || strings.HasPrefix(block, "REGION") {
			continue
		}
		lines := strings.Split(block, "\n")
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
		times := convertStamp(lines[stamp])
		if times == "" {
			continue
		}
		text := strings.TrimSpace(strings.Join(lines[stamp+1:], "\n"))
		if text == "" {
			continue
		}
		cues = append(cues, times+"\n"+text)
	}
	if len(cues) == 0 {
		return nil, errors.New("no cues")
	}
	var b strings.Builder
	for i, cue := range cues {
		fmt.Fprintf(&b, "%d\n%s\n\n", i+1, cue)
	}
	return []byte(b.String()), nil
}

func convertStamp(line string) string {
	line = strings.TrimSpace(line)
	left, right, ok := strings.Cut(line, "-->")
	if !ok {
		return ""
	}
	from := vttTime(strings.TrimSpace(left))
	right = strings.TrimSpace(right)
	if i := strings.IndexAny(right, " \t"); i >= 0 {
		right = right[:i]
	}
	to := vttTime(right)
	if from == "" || to == "" {
		return ""
	}
	return from + " --> " + to
}

func vttTime(t string) string {
	t = strings.TrimSpace(t)
	t = strings.Replace(t, ".", ",", 1)
	n := strings.Count(t, ":")
	switch n {
	case 1:
		return "00:" + t
	case 2:
		return t
	default:
		return ""
	}
}
