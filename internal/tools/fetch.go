package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Platform is a GOOS and GOARCH pair.
type Platform struct {
	OS, Arch string
}

// Host is the platform Snag runs on.
func Host() Platform {
	return Platform{runtime.GOOS, runtime.GOARCH}
}

// String returns "os/arch".
func (p Platform) String() string { return p.OS + "/" + p.Arch }

func (p Platform) exe(name string) string {
	if p.OS == "windows" {
		return name + ".exe"
	}
	return name
}

// YtDLPVersion is the yt-dlp release Snag ships; the app updates it afterwards.
const YtDLPVersion = "2026.08.19"

const ytdlpReleases = "https://github.com/yt-dlp/yt-dlp/releases/download/"

var ytdlpAssets = map[string]string{
	"linux/amd64":   "yt-dlp_linux",
	"linux/arm64":   "yt-dlp_linux_aarch64",
	"darwin/amd64":  "yt-dlp_macos",
	"darwin/arm64":  "yt-dlp_macos",
	"windows/amd64": "yt-dlp.exe",
	"windows/arm64": "yt-dlp_arm64.exe",
}

// FetchYtDLP downloads the pinned yt-dlp for p into dir, verified against the release's published
// checksums, and returns its path. A matching existing copy is kept.
func FetchYtDLP(ctx context.Context, dir string, p Platform) (string, error) {
	asset, ok := ytdlpAssets[p.String()]
	if !ok {
		return "", fmt.Errorf("no yt-dlp build for %s", p)
	}
	path := filepath.Join(dir, p.exe("yt-dlp"))
	release := ytdlpReleases + YtDLPVersion + "/"
	want, err := publishedChecksum(ctx, release+"SHA2-256SUMS", asset)
	if err != nil {
		return "", err
	}
	if have, err := os.ReadFile(path); err == nil && sha256Hex(have) == want {
		return path, nil
	}
	data, err := download(ctx, release+asset, want)
	if err != nil {
		return "", err
	}
	return path, writeExecutable(path, data)
}

func download(ctx context.Context, url, sha256Sum string) ([]byte, error) {
	data, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	if got := sha256Hex(data); got != sha256Sum {
		return nil, fmt.Errorf("%s: checksum %s, want %s", url, got, sha256Sum)
	}
	return data, nil
}

// publishedChecksum finds a file's line ("<sha256>  <name>") in a release's checksum file.
func publishedChecksum(ctx context.Context, sumsURL, name string) (string, error) {
	sums, err := get(ctx, sumsURL)
	if err != nil {
		return "", err
	}
	return checksumFor(sums, name)
}

func checksumFor(sums []byte, name string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum for %s", name)
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// writeExecutable replaces path only once the new file is complete.
func writeExecutable(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".download"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
