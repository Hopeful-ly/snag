// Command sign-feed signs Snag's update feed with the Ed25519 seed in $SNAG_FEED_KEY (base64).
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
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
	out := flag.String("o", "feed.json", "where to write the feed")
	flag.Parse()
	if err := sign(*version, *page, *ytdlp, *out); err != nil {
		log.Fatal(err)
	}
}

func sign(version, page, ytdlp, out string) error {
	if (version == "") != (page == "") {
		return errors.New("give both the latest version of Snag (-snag) and its page (-page), or neither")
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
