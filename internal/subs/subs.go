// Package subs attaches captions to a finished video, muxed in or as a sidecar .srt.
package subs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/platform"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"
	maxBytes  = 5 << 20
)

// Request describes captions to put on a finished video.
type Request struct {
	URL     string
	Text    string // cues already in hand, as .srt or .vtt; takes precedence over URL
	Video   string
	Lang    string // such as "en"; used in the sidecar name
	Sidecar bool
	Clip    clip.Range // the clip Video holds; captions are cut to match
	Headers map[string]string
	Cookies []cookies.Cookie
	FFmpeg  string
	WorkDir string
}

// Attach fetches the caption file and either muxes it into the video or writes a .srt next to it.
func Attach(ctx context.Context, r Request) error {
	if r.Video == "" || (r.URL == "" && r.Text == "") {
		return nil
	}
	if err := os.MkdirAll(r.WorkDir, 0o700); err != nil {
		return err
	}
	raw := filepath.Join(r.WorkDir, "captions.bin")
	if r.Text != "" {
		if err := os.WriteFile(raw, []byte(r.Text), 0o644); err != nil {
			return err
		}
	} else if err := fetch(ctx, r, raw); err != nil {
		return err
	}
	srt := filepath.Join(r.WorkDir, "captions.srt")
	if err := toSRT(r.ffmpeg(), raw, srt); err != nil {
		return err
	}
	if r.Clip.IsSet() {
		data, err := os.ReadFile(srt)
		if err != nil {
			return err
		}
		data = cutSRT(data, r.Clip)
		if len(data) == 0 {
			return nil // no cues within the clip
		}
		if err := os.WriteFile(srt, data, 0o644); err != nil {
			return err
		}
	}
	if r.Sidecar || r.ffmpeg() == "" {
		return copyFile(srt, sidecarPath(r.Video, r.Lang))
	}
	if err := embed(ctx, r.ffmpeg(), r.Video, srt, r.Lang); err != nil {
		if copyErr := copyFile(srt, sidecarPath(r.Video, r.Lang)); copyErr != nil {
			return fmt.Errorf("embed captions: %w (and sidecar: %w)", err, copyErr)
		}
		return nil
	}
	return nil
}

func (r Request) ffmpeg() string {
	if r.FFmpeg != "" {
		return r.FFmpeg
	}
	path, _ := exec.LookPath("ffmpeg")
	return path
}

func sidecarPath(video, lang string) string {
	stem := strings.TrimSuffix(video, filepath.Ext(video))
	code := strings.ToLower(strings.TrimSpace(lang))
	code, _, _ = strings.Cut(code, "-")
	if code == "" || code == "und" {
		return stem + ".srt"
	}
	return stem + "." + code + ".srt"
}

func fetch(ctx context.Context, r Request, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	for name, value := range r.Headers {
		switch strings.ToLower(name) {
		case "range", "accept-encoding", "host", "content-length", "connection":
			continue
		}
		req.Header.Set(name, value)
	}
	if u := req.URL; u != nil {
		if cookie := cookies.Header(r.Cookies, u); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("captions: server said %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxBytes {
		return errors.New("caption file is too large")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return errors.New("caption file is empty")
	}
	return os.WriteFile(dest, body, 0o644)
}

func toSRT(ffmpeg, src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if looksSRT(data) {
		return copyFile(src, dest)
	}
	if srt, err := vttToSRT(data); err == nil {
		return os.WriteFile(dest, srt, 0o644)
	}
	if ffmpeg == "" {
		return errors.New("need ffmpeg to convert these captions")
	}
	cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-i", src, dest)
	platform.StartGroup(cmd) // no console window on Windows
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("convert captions: %w (%s)", err, bytes.TrimSpace(out))
	}
	return nil
}

func embed(ctx context.Context, ffmpeg, video, srt, lang string) error {
	ext := strings.ToLower(filepath.Ext(video))
	tmp := video + ".snag-subs" + ext
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", video, "-i", srt,
		"-map", "0", "-map", "1",
		"-c", "copy", "-c:s", subtitleCodec(ext),
	}
	if code := ffmpegLang(lang); code != "" {
		args = append(args, "-metadata:s:s:0", "language="+code)
	}
	args = append(args, tmp)
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	platform.StartGroup(cmd) // no console window on Windows
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("mux captions: %w (%s)", err, bytes.TrimSpace(out))
	}
	if err := os.Rename(tmp, video); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func subtitleCodec(ext string) string {
	switch ext {
	case ".mkv":
		return "srt"
	case ".webm":
		return "webvtt"
	default:
		return "mov_text"
	}
}

func ffmpegLang(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code, _, _ = strings.Cut(code, "-")
	if code == "" || code == "und" {
		return ""
	}
	return code
}

func looksSRT(data []byte) bool {
	s := string(bytes.TrimLeftFunc(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}), unicode.IsSpace))
	if strings.HasPrefix(s, "WEBVTT") {
		return false
	}
	return strings.Contains(s, "-->") && strings.Contains(s, ",")
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
