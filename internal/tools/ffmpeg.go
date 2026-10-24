package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// The Windows FFmpeg: BtbN's LGPL "shared" build (redistributable with credit and a source link),
// copied to a release of Snag's own because BtbN deletes its nightly builds after a few weeks.
const (
	ffmpegRelease = "https://github.com/Roham1299/snag/releases/download/ffmpeg-n9.0.2-35-g9a9b70bb8c/"
	ffmpegBuild   = "ffmpeg-n9.0.2-35-g9a9b70bb8c"
	ffmpegChannel = "lgpl-shared-9.0"
)

// ffmpegAssets are the archives for each platform, with their SHA-256.
var ffmpegAssets = map[string]struct{ name, sha256 string }{
	"windows/amd64": {ffmpegBuild + "-win64-" + ffmpegChannel + ".zip", "fb771d872989dba5d96621a840194362202426e882814a0d5e7eddd8387908d3"},
	"windows/arm64": {ffmpegBuild + "-winarm64-" + ffmpegChannel + ".zip", "4bf8f5ff4ec2ec462c50d0c65f77793c2e18303cb343ef376c8c020c7028d248"},
}

// ErrNoFFmpegBuild is returned for a platform Snag brings no FFmpeg for: it uses the system's.
var ErrNoFFmpegBuild = errors.New("no FFmpeg build to ship")

// FetchFFmpeg downloads the pinned FFmpeg build for p and puts ffmpeg, ffprobe and their libraries in dir.
func FetchFFmpeg(ctx context.Context, dir string, p Platform) error {
	asset, ok := ffmpegAssets[p.String()]
	if !ok {
		return fmt.Errorf("%w for %s", ErrNoFFmpegBuild, p)
	}
	archive, err := download(ctx, ffmpegRelease+asset.name, asset.sha256)
	if err != nil {
		return err
	}
	programs := map[string]string{p.exe("ffmpeg"): "", p.exe("ffprobe"): ""}
	found, err := extract(archive, strings.HasSuffix(asset.name, ".zip"), programs, ".dll")
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
