package tools

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestChecksumForAcceptsBinaryMarks(t *testing.T) {
	sums := []byte("aa11 *ffmpeg.zip\nbb22  other.zip\n")
	if got, err := checksumFor(sums, "ffmpeg.zip"); err != nil || got != "aa11" {
		t.Errorf("checksumFor = %q, %v", got, err)
	}
}

func TestExtractTakesTheProgramsFromBin(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"ffmpeg-x/bin/ffmpeg.exe":     "ffmpeg",
		"ffmpeg-x/bin/ffprobe.exe":    "ffprobe",
		"ffmpeg-x/bin/avcodec-62.dll": "library",
		"ffmpeg-x/doc/ffmpeg.exe":     "not this one",
		"ffmpeg-x/LICENSE.txt":        "lgpl",
	} {
		w, _ := z.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = z.Close()

	found, err := extract(buf.Bytes(), true, map[string]string{"ffmpeg.exe": "", "ffprobe.exe": ""}, ".dll")
	if err != nil {
		t.Fatal(err)
	}
	if string(found["ffmpeg.exe"]) != "ffmpeg" || string(found["ffprobe.exe"]) != "ffprobe" ||
		string(found["avcodec-62.dll"]) != "library" || len(found) != 3 {
		t.Errorf("found %v", found)
	}
	if _, err := extract(buf.Bytes(), true, map[string]string{"ffplay.exe": ""}, ""); err == nil {
		t.Error("a missing program wasn't reported")
	}
}

func TestEveryShippedPlatformHasItsTools(t *testing.T) {
	for _, p := range []string{"linux/amd64", "darwin/arm64", "windows/amd64"} {
		if ytdlpAssets[p] == "" {
			t.Errorf("%s lacks a yt-dlp build", p)
		}
	}
	if ffmpegAssets["windows/amd64"] == "" {
		t.Error("Windows lacks an FFmpeg build")
	}
}
