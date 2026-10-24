package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// The Windows FFmpeg: BtbN's LGPL build (redistributable with credit and a source link), the
// "shared" variant because it is half the size. Linux packages use the system's FFmpeg.
const (
	ffmpegRelease  = "https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-10-07-13-07/"
	ffmpegBuild    = "ffmpeg-n9.0.2-22-g46d8f462ee"
	ffmpegChannel  = "lgpl-shared-9.0"
	ffmpegChecksum = "checksums.sha256"
)

var ffmpegAssets = map[string]string{
	"windows/amd64": ffmpegBuild + "-win64-" + ffmpegChannel + ".zip",
	"windows/arm64": ffmpegBuild + "-winarm64-" + ffmpegChannel + ".zip",
}

// FetchFFmpeg downloads the pinned FFmpeg build for p and puts ffmpeg, ffprobe and their libraries in dir.
func FetchFFmpeg(ctx context.Context, dir string, p Platform) error {
	asset, ok := ffmpegAssets[p.String()]
	if !ok {
		return fmt.Errorf("no FFmpeg build to ship for %s", p)
	}
	want, err := publishedChecksum(ctx, ffmpegRelease+ffmpegChecksum, asset)
	if err != nil {
		return err
	}
	archive, err := download(ctx, ffmpegRelease+asset, want)
	if err != nil {
		return err
	}
	programs := map[string]string{p.exe("ffmpeg"): "", p.exe("ffprobe"): ""}
	found, err := extract(archive, strings.HasSuffix(asset, ".zip"), programs, ".dll")
	if err != nil {
		return err
	}
	for name, data := range found {
		if err := writeExecutable(filepath.Join(dir, name), data); err != nil {
			return err
		}
	}
	return nil
}

// extract returns the files in want, plus those ending in alsoEnding, from the archive's bin folder.
func extract(archive []byte, isZip bool, want map[string]string, alsoEnding string) (map[string][]byte, error) {
	found := map[string][]byte{}
	take := func(name string, open func() (io.Reader, error)) error {
		base := path.Base(name)
		_, wanted := want[base]
		library := alsoEnding != "" && strings.HasSuffix(base, alsoEnding)
		if path.Base(path.Dir(name)) != "bin" || !wanted && !library {
			return nil
		}
		r, err := open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(r)
		found[base] = data
		return err
	}

	if isZip {
		z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range z.File {
			err := take(f.Name, func() (io.Reader, error) { return f.Open() })
			if err != nil {
				return nil, err
			}
		}
	} else {
		xr, err := xz.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(xr)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if err := take(h.Name, func() (io.Reader, error) { return tr, nil }); err != nil {
				return nil, err
			}
		}
	}
	for name := range want {
		if found[name] == nil {
			return nil, fmt.Errorf("%s isn't in the FFmpeg archive", name)
		}
	}
	return found, nil
}
