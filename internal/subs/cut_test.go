package subs

import (
	"testing"

	"github.com/Roham1299/snag/internal/clip"
)

func TestCutSRT(t *testing.T) {
	srt := "1\n00:00:01,360 --> 00:00:03,040\n[music]\n\n" +
		"2\r\n00:01:01,500 --> 00:01:04,000\r\nbefore and in\r\n\r\n" +
		"3\n00:01:05,000 --> 00:01:08,250\nin\nsecond line\n\n" +
		"4\n00:01:09,000 --> 00:01:12,000\nin and after\n\n" +
		"5\n00:01:20,000 --> 00:01:22,000\nafter\n"
	want := "1\n00:00:00,000 --> 00:00:02,000\nbefore and in\n\n" +
		"2\n00:00:03,000 --> 00:00:06,250\nin\nsecond line\n\n" +
		"3\n00:00:07,000 --> 00:00:08,000\nin and after\n\n"
	if got := string(cutSRT([]byte(srt), clip.Range{From: 62, To: 70})); got != want {
		t.Errorf("cutSRT =\n%q\nwant\n%q", got, want)
	}
}
