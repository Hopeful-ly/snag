package probe

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/httpdl"
)

// Kinds of streaming playlist.
const (
	KindHLS  = "hls"
	KindDASH = "dash"
)

// Sniffed is what the start of an address turned out to be.
type Sniffed struct {
	Kind string // KindHLS, KindDASH, or ""
	Text string // the playlist's start, for the extension to read variants from
}

var dashStart = regexp.MustCompile(`<MPD[\s>]`)

// Sniff detects a streaming playlist by content, regardless of its name, reading only its start.
func (p *Prober) Sniff(ctx context.Context, address string, headers map[string]string, jar []cookies.Cookie) Sniffed {
	text, _, err := p.get(ctx, address, headers, jar, 512<<10)
	if err != nil {
		return Sniffed{}
	}
	text = strings.ToValidUTF8(text, "\uFFFD")
	keep := text[:min(len(text), 256<<10)]
	switch {
	case strings.HasPrefix(strings.TrimLeft(text, "\uFEFF \r\n\t"), "#EXTM3U"):
		return Sniffed{Kind: KindHLS, Text: keep}
	case dashStart.MatchString(text[:min(len(text), 4096)]):
		return Sniffed{Kind: KindDASH, Text: keep}
	}
	return Sniffed{}
}

var byteRange = regexp.MustCompile(`#EXT-X-BYTERANGE:\s*(\d+)`)

// estimateHLSSize is exact for byte-range playlists, else extrapolated from three sampled pieces;
// 0 when unknown.
func (p *Prober) estimateHLSSize(ctx context.Context, playlist string, headers map[string]string, jar []cookies.Cookie) int64 {
	text, base, err := p.get(ctx, playlist, headers, jar, 4<<20)
	if err != nil || strings.Contains(text, "#EXT-X-STREAM-INF") {
		return 0
	}
	if ranges := byteRange.FindAllStringSubmatch(text, -1); len(ranges) > 0 {
		var total int64
		for _, m := range ranges {
			n, _ := strconv.ParseInt(m[1], 10, 64)
			total += n
		}
		return total
	}

	var pieces []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			pieces = append(pieces, line)
		}
	}
	if len(pieces) == 0 {
		return 0
	}
	samples := map[int]bool{len(pieces) / 4: true, len(pieces) / 2: true, 3 * len(pieces) / 4: true}
	var sum, counted int64
	for i := range samples {
		ref, err := url.Parse(pieces[i])
		if err != nil {
			continue
		}
		if size := p.pieceSize(ctx, base.ResolveReference(ref).String(), headers, jar); size > 0 {
			sum += size
			counted++
		}
	}
	if counted == 0 {
		return 0
	}
	return sum / counted * int64(len(pieces))
}

// pieceSize requests one byte to read the size from Content-Range, falling back to a HEAD.
func (p *Prober) pieceSize(ctx context.Context, address string, headers map[string]string, jar []cookies.Cookie) int64 {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req, err := p.request(ctx, method, address, headers, jar)
		if err != nil {
			return 0
		}
		if method == http.MethodGet {
			req.Header.Set("Range", "bytes=0-0")
		}
		resp, err := p.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64); err == nil && n > 0 {
				return n
			}
		}
		if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
			return resp.ContentLength
		}
	}
	return 0
}

// get reads up to limit bytes of address and returns them with the post-redirect URL.
func (p *Prober) get(ctx context.Context, address string, headers map[string]string, jar []cookies.Cookie, limit int64) (string, *url.URL, error) {
	req, err := p.request(ctx, http.MethodGet, address, headers, jar)
	if err != nil {
		return "", nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return "", nil, &httpdl.StatusError{Code: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return string(data), resp.Request.URL, err
}

// request builds a request with the referer, origin, user agent and cookies sites check.
func (p *Prober) request(ctx context.Context, method, address string, headers map[string]string, jar []cookies.Cookie) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpdl.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	for name, value := range headers {
		switch strings.ToLower(name) {
		case "referer", "user-agent", "origin":
			req.Header.Set(name, value)
		}
	}
	if referer := req.Header.Get("Referer"); referer != "" && req.Header.Get("Origin") == "" {
		if u, err := url.Parse(referer); err == nil {
			req.Header.Set("Origin", u.Scheme+"://"+u.Host)
		}
	}
	if cookie := cookies.Header(jar, req.URL); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	return req, nil
}
