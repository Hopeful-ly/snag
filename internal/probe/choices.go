package probe

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Roham1299/snag/internal/ytdlp"
)

// Choice is one quality offered for download.
type Choice struct {
	Label  string `json:"label"`  // "1080p60", "Audio"
	Detail string `json:"detail"` // "MP4 · HDR"
	// Format and Sort are yt-dlp's format selector and sort order.
	Format string `json:"format"`
	Sort   string `json:"sort,omitempty"`
	Size   int64  `json:"size"` // estimated; 0 when unknown
	Height int    `json:"height,omitempty"`
	// Audio is a format to convert to, such as "mp3".
	AudioOnly bool   `json:"audioOnly,omitempty"`
	Audio     string `json:"audio,omitempty"`

	playlist string // a stream variant's playlist, to estimate its size from
}

func hasVideo(f ytdlp.Format) bool  { return f.VCodec != "" && f.VCodec != "none" }
func hasAudio(f ytdlp.Format) bool  { return f.ACodec != "" && f.ACodec != "none" }
func audioOnly(f ytdlp.Format) bool { return f.VCodec == "none" && hasAudio(f) }

func formatsOf(info ytdlp.Info) []ytdlp.Format {
	if len(info.Formats) > 0 {
		return info.Formats
	}
	return []ytdlp.Format{info.Format}
}

// sizeOf is a format's stated size, else estimated from its bitrate.
func sizeOf(f ytdlp.Format, duration float64) int64 {
	size := f.Filesize
	if size == 0 {
		size = f.FilesizeApprox
	}
	if size == 0 && f.TBR > 0 && duration > 0 {
		size = f.TBR * 1000 / 8 * duration
	}
	return int64(size)
}

func bestAudio(formats []ytdlp.Format) (ytdlp.Format, bool) {
	var best ytdlp.Format
	found := false
	for _, f := range formats {
		if audioOnly(f) && (!found || max(f.ABR, f.TBR) > max(best.ABR, best.TBR)) {
			best, found = f, true
		}
	}
	return best, found
}

// pageChoices offers one choice per height and frame rate, best first, then M4A and MP3 audio.
func pageChoices(info ytdlp.Info) []Choice {
	formats := formatsOf(info)
	audio, haveAudio := bestAudio(formats)
	type variant struct {
		height int
		smooth bool // over 30 fps
	}
	var variants []variant
	videos := map[variant][]ytdlp.Format{}
	for _, f := range formats {
		if hasVideo(f) && f.Height > 0 {
			v := variant{int(f.Height), f.FPS > 30}
			if videos[v] == nil {
				variants = append(variants, v)
			}
			videos[v] = append(videos[v], f)
		}
	}
	slices.SortFunc(variants, func(a, b variant) int {
		if a.height != b.height {
			return b.height - a.height
		}
		return boolInt(b.smooth) - boolInt(a.smooth)
	})

	var choices []Choice
	for _, v := range variants {
		top := slices.MaxFunc(videos[v], func(a, b ytdlp.Format) int {
			return compareAll(float64(boolInt(a.Ext == "mp4")-boolInt(b.Ext == "mp4")), a.FPS-b.FPS, a.TBR-b.TBR)
		})
		size := sizeOf(top, info.Duration)
		if !hasAudio(top) && haveAudio {
			size += sizeOf(audio, info.Duration)
		}
		label, rate := fmt.Sprintf("%dp", v.height), "[fps<=30]"
		if v.smooth {
			label, rate = fmt.Sprintf("%dp%d", v.height, int(top.FPS)), "[fps>30]"
		}
		detail := []string{strings.ToUpper(top.Ext)}
		if top.DynamicRange != "" && top.DynamicRange != "SDR" {
			detail = append(detail, top.DynamicRange)
		}
		h := v.height
		choices = append(choices, Choice{
			Label:  label,
			Detail: strings.Join(detail, " · "),
			Format: fmt.Sprintf("bv*[height=%d]%s+ba/b[height=%d]%s/bv*[height=%d]+ba/bv*+ba/b", h, rate, h, rate, h),
			Sort:   fmt.Sprintf("res:%d,fps,vcodec:h264,acodec:aac,ext:mp4:m4a", h),
			Size:   size,
			Height: h,
		})
	}
	if len(choices) == 0 {
		choices = append(choices, Choice{Label: "Best", Detail: strings.ToUpper(info.Ext), Format: "bv*+ba/b",
			Size: sizeOf(info.Format, info.Duration)})
	}
	if haveAudio || len(variants) > 0 {
		audioSize := int64(0)
		if haveAudio {
			audioSize = sizeOf(audio, info.Duration)
		}
		choices = append(choices,
			Choice{Label: "Audio", Detail: "M4A", Format: "ba[ext=m4a]/ba/b", Size: audioSize, AudioOnly: true, Audio: "m4a"},
			Choice{Label: "Audio", Detail: "MP3", Format: "ba/b", Size: audioSize, AudioOnly: true, Audio: "mp3"},
		)
	}
	return choices
}

var resolutionInPath = regexp.MustCompile(`(?:^|[^0-9])(2160|1440|1080|720|576|540|480|360|240)p`)

// streamChoices lists every variant of a streaming playlist (they can differ by more than
// height), best first, then its audio.
func streamChoices(info ytdlp.Info, address string) []Choice {
	formats := formatsOf(info)
	audio, haveAudio := bestAudio(formats)
	var videos []ytdlp.Format
	for _, f := range formats {
		if f.VCodec != "none" && (f.Height > 0 || f.TBR > 0) {
			videos = append(videos, f)
		}
	}
	slices.SortStableFunc(videos, func(a, b ytdlp.Format) int { return compareAll(b.Height-a.Height, b.TBR-a.TBR) })

	var choices []Choice
	for _, f := range videos {
		size := sizeOf(f, info.Duration)
		if haveAudio && !hasAudio(f) {
			size += sizeOf(audio, info.Duration)
		}
		detail := []string{strings.ToUpper(f.Ext)}
		if f.TBR > 0 {
			detail = append(detail, fmt.Sprintf("%.1f Mbps", f.TBR/1000))
		}
		if f.FPS > 30 {
			detail = append(detail, fmt.Sprintf("%d fps", int(f.FPS)))
		}
		label := "Video"
		if f.Height > 0 {
			label = fmt.Sprintf("%dp", int(f.Height))
		}
		choices = append(choices, Choice{
			Label: label, Detail: strings.Join(detail, " · "),
			Format: f.FormatID + "+ba/" + f.FormatID, Size: size, Height: int(f.Height), playlist: f.URL,
		})
	}
	if len(choices) == 0 {
		// No stated resolution; the address often has one (".../index-1080p-...").
		choice := Choice{Label: "Stream", Format: "bv*+ba/b", Size: sizeOf(info.Format, info.Duration), playlist: address}
		if u, err := url.Parse(address); err == nil {
			if m := resolutionInPath.FindStringSubmatch(u.Path); m != nil {
				choice.Label = m[1] + "p"
				choice.Height, _ = strconv.Atoi(m[1])
			}
		}
		detail := []string{"MP4"}
		switch {
		case info.Duration >= 60:
			detail = append(detail, fmt.Sprintf("%.0f min", info.Duration/60))
		case info.Duration > 0:
			detail = append(detail, fmt.Sprintf("%.0f s", info.Duration))
		}
		choice.Detail = strings.Join(detail, " · ")
		choices = append(choices, choice)
	}
	if haveAudio {
		choices = append(choices, Choice{Label: "Audio", Detail: "M4A", Format: "ba",
			Size: sizeOf(audio, info.Duration), AudioOnly: true, Audio: "m4a"})
	}
	return choices
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// compareAll returns the sign of the first nonzero difference.
func compareAll(diffs ...float64) int {
	for _, d := range diffs {
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
	}
	return 0
}
