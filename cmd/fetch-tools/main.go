// Command fetch-tools downloads the pinned, checksum-verified yt-dlp and FFmpeg that Snag ships with.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/Roham1299/snag/internal/tools"
)

func main() {
	host := tools.Host()
	goos := flag.String("os", host.OS, "the OS to fetch for")
	arch := flag.String("arch", host.Arch, "the processor to fetch for")
	flag.Parse()
	dir := "bin/tools"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	if err := fetch(dir, tools.Platform{OS: *goos, Arch: *arch}); err != nil {
		log.Fatal(err)
	}
}

func fetch(dir string, p tools.Platform) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	path, err := tools.FetchYtDLP(ctx, dir, p)
	if err != nil {
		return err
	}
	fmt.Println(path)
	if err := tools.FetchFFmpeg(ctx, dir, p); err != nil {
		fmt.Println("no FFmpeg:", err)
		return nil
	}
	fmt.Println("ffmpeg and ffprobe in", dir)
	return nil
}
