# Snag

**A fast download manager that lives in your browser.** Hover over any video and a Download button
appears on it: pick a quality and Snag saves it, properly named, at full speed. Files you download
in the browser go through Snag too, split across several connections and resumable at any time.

Inspired by the browser integration of Internet Download Manager (a button on videos, catching
downloads, one window for everything) and rebuilt for today's web: adaptive streams, every quality
to choose from, and a clean, modern interface.

## Features

**On every video**
- A **Download** button appears on videos you hover or start playing. Drag it anywhere on the
  video; each site remembers its spot.
- One list of everything the page offers: every quality from highest to lowest (4K, Full HD, HD,
  SD) with sizes, audio-only versions (M4A, MP3), and any other files on the page.
- Works on YouTube and about 1,800 other video sites, and on any page that plays a plain video
  file or an HLS (`.m3u8`) / DASH (`.mpd`) stream, even when the stream's address is disguised.
  Ads, previews and subtitle tracks stay out of the list.
- Clips: switch the card to **Clip** and a trim bar appears over the video. Drag its ends, or set
  them to where the video is, and only that part downloads, cut to the exact frame.
- Proper names: a series episode is saved as `Series S01E03 - Episode name.mp4`, a film as
  `Title (2024).mp4`; files keep the name their server gives them.

**Downloads**
- Several connections per file (8 by default), dynamically re-split as parts finish, for the full
  speed of your line.
- Pause and resume, also after a restart or a lost connection; stream links that expire are picked
  up again when you click Download on the same video.
- Watch while it downloads: open the file from your downloads folder and play what has arrived.
- Downloads from your browser (archives, programs, videos, documents, anything large) are handed
  to Snag automatically; if Snag can't take one, the browser gets it back.
- Many links at once: right-click a page for "Download all links with Snag…" (or select part of
  it first), paste several links, or import a file of them. Snag groups them (archives,
  programs, videos, documents…), learns their real names and sizes, and downloads the ones you
  tick, keeping the site's folders if you like.
- Checksums: when the page, the server, GitHub or a checksum file next to it says what a file's
  checksum is, Snag offers to check the finished file, and to download it again if it doesn't
  match. Paste one to check any file.

**Subscriptions**
- Follow a YouTube channel or playlist, a podcast, or a Spotify playlist: Snag looks for what's
  new every few hours and downloads it the way you chose, oldest first, each thing once. Leave
  out Shorts, short videos, or titles with certain words.
- Subscribe from the Download card on the channel's or podcast's page, with "Keep downloading
  new ones" in the playlist window, or by pasting its address on the Subscriptions screen.
- The Subscriptions screen shows what each one follows, how it downloads, when Snag last looked
  and will look next, and what went wrong; check, pause, edit or unsubscribe from there. Click
  one in the sidebar to see what it downloaded.

**The Snag app**
- Every download in one window: filters, categories, search, live progress, speed and time left.
- A Save dialog for each new download (name, folder, recent folders), or straight to your
  downloads folder if you prefer.
- A tray icon with what is downloading and quick pause/resume.
- Starts with your computer, in the tray, so queues and subscriptions run on time (turn it off in
  Settings).
- Every action shows it is happening; nothing freezes.

**Private by design.** Snag runs entirely on your computer. The extension talks only to the Snag
app on the same machine. Once a day the app reads a small file on Snag's website, signed by
Snag, to learn of a new release and which yt-dlp to use; it sends nothing about you or your
downloads.

## Platforms

| | Linux | Windows | macOS |
|---|---|---|---|
| Snag app | ✓ | ✓ (installer, early) | ✓ (disk image, early) |
| Browser extension (Chrome, Edge, Brave, Vivaldi, Opera, Firefox) | ✓ | ✓ | ✓ |

Snag is a Go program with a web-view window ([Wails](https://v3.wails.io/)), built for all three
from one source. The Windows and macOS builds are installed and checked automatically on every
release, but have seen little use on real machines yet, and aren't code-signed: Windows'
SmartScreen and macOS' Gatekeeper ask before the first start. Snag is distributed from its own
website: the app carries the browser extension and walks you through adding it to your browser.

## Limitations

- Videos protected with DRM (Netflix, Disney+, Spotify and similar) can't be downloaded.
- Some sites change often. Snag keeps its video engine (yt-dlp) up to date by itself, which almost
  always fixes a site that stopped working.

## Install

**From a release:** the Windows installer, the Linux package (`.deb`, `.rpm`, Arch) or the macOS
disk image. Start Snag once; it sets itself up for your browsers, and its "Connect your browser"
screen shows how to add the extension (Developer mode, "Load unpacked", pick the folder it shows).
For Firefox, open [snagdl.com/firefox](https://snagdl.com/firefox) in Firefox: the add-on is signed
by Mozilla and updates itself.
Snag updates itself too: when a new version is out, it downloads it and offers to restart into
it.

**From source (Linux):** with the tools in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md),

```sh
task setup && task tools
task install:local   # into ~/.local/lib/snag, and points your browsers at it
```

Nothing else to start: the browser starts Snag when it needs it. Open the app from your launcher,
the tray icon, or "Open Snag" in the extension.

## Project layout

- `cmd/snag`: the app; `cmd/snag-host`: the small helper browsers start to reach it.
- `internal/`: the download engine, the runners for files and streams, probing pages, the
  connection to the extension, and what differs per OS.
- `frontend/`: the app's window (Svelte + TypeScript).
- `extension/`: the browser extension (TypeScript, built for Chromium and Firefox).
- `packaging/`: the installers.
- [PROTOCOL.md](PROTOCOL.md): how the extension talks to the app.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): architecture, tests, and notes for contributors.
- [docs/RELEASING.md](docs/RELEASING.md): making a release, and what needs your own accounts.

## License

Snag is free software: you can use, study, change and share it under the
[GNU General Public License, version 3 or later](LICENSE). It comes with no warranty.

The name "Snag" and its logo aren't covered by the license: if you publish a changed version,
give it another name.

Snag uses [yt-dlp](https://github.com/yt-dlp/yt-dlp) (public domain) and, in the Windows
installer, an LGPL build of [FFmpeg](https://ffmpeg.org) from
[BtbN's FFmpeg-Builds](https://github.com/BtbN/FFmpeg-Builds), pinned in `internal/tools/ffmpeg.go`.
