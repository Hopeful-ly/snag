// Command sign-feed signs Snag's update feed with the Ed25519 seed in $SNAG_FEED_KEY (base64).
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/feed"
)

func main() {
	version := flag.String("snag", "", "the latest version of Snag, such as 1.2.0; empty for none yet")
	page := flag.String("page", "", "the https page to get that version from")
	ytdlp := flag.String("ytdlp", "", "the yt-dlp to use, as --update-to takes it; empty for its latest")
	sums := flag.String("sums", "", "the release's SHA256SUMS, for the installers Snag updates itself from")
	download := flag.String("download", "", "the https address the release's files are under")
	out := flag.String("o", "feed.json", "where to write the feed")
	flag.Parse()
	if err := sign(*version, *page, *ytdlp, *sums, *download, *out); err != nil {
		log.Fatal(err)
	}
}

func sign(version, page, ytdlp, sums, download, out string) error {
	if (version == "") != (page == "") {
		return errors.New("give both the latest version of Snag (-snag) and its page (-page), or neither")
	}
	if (sums == "") != (download == "") || (sums != "" && version == "") {
		return errors.New("give the release's SHA256SUMS (-sums) with where its files are (-download), and its version")
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("SNAG_FEED_KEY")))
	if err != nil || len(seed) != ed25519.SeedSize {
		return errors.New("$SNAG_FEED_KEY isn't the feed's private key")
	}
	key := ed25519.NewKeyFromSeed(seed)
	public, _ := base64.StdEncoding.DecodeString(feed.PublicKey)
	if !key.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(public)) {
		return errors.New("$SNAG_FEED_KEY isn't the key Snag trusts (feed.PublicKey)")
	}

	f := feed.Feed{Issued: time.Now().UTC().Truncate(time.Second), YtDLP: ytdlp}
	if version != "" {
		f.Snag = &feed.Release{Version: strings.TrimPrefix(version, "v"), Page: page}
	}
	if sums != "" {
		files, err := installers(sums, download)
		if err != nil {
			return err
		}
		f.Snag.Files = files
	}
	data, err := feed.Seal(f, key)
	if err != nil {
		return err
	}
	// Never publish a feed Snag would refuse.
	if _, err := feed.Open(data, public); err != nil {
		return err
	}
	return os.WriteFile(out, append(data, '\n'), 0o644)
}

// installers reads a SHA256SUMS ("<hash>  <name>" lines) and returns the installers in it.
func installers(sums, download string) (map[string]feed.File, error) {
	data, err := os.ReadFile(sums)
	if err != nil {
		return nil, err
	}
	files := map[string]feed.File{}
	for line := range strings.Lines(string(data)) {
		hash, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		name = strings.TrimLeft(name, " *")
		if !ok || name == "" {
			continue
		}
		if kind := feed.FileKind(name); kind != "" {
			files[kind] = feed.File{URL: strings.TrimSuffix(download, "/") + "/" + name, SHA256: strings.ToLower(hash)}
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s names no installers", sums)
	}
	return files, nil
}
