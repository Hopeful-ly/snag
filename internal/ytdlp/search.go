package ytdlp

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Roham1299/snag/internal/platform"
)

// SearchArgs returns arguments that fully describe the first n results of query, which is a
// search URL ("https://music.youtube.com/search?q=…#songs") or a yt-dlp search ("ytsearch:…").
func SearchArgs(query string, n int, p Programs) []string {
	args := common(p)
	return append(args, "-J", "--socket-timeout", "15", "--yes-playlist",
		"--playlist-items", "1:"+strconv.Itoa(max(1, n)), "--", query)
}

// Search runs yt-dlp with SearchArgs.
func Search(ctx context.Context, ytdlp string, p Programs, query string, n int) ([]Info, error) {
	cmd := exec.CommandContext(ctx, ytdlp, SearchArgs(query, n, p)...)
	platform.StartGroup(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, describeError(stderr.String(), err)
	}
	info, err := readInfo(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	var results []Info
	for _, e := range info.Entries {
		if strings.TrimSpace(e.WebpageURL) != "" {
			results = append(results, e)
		}
	}
	return results, nil
}
