# Developing Snag

## How it fits together

```
page ── content.js (button, card) ── background.js ── native messaging ── snag-host ── socket ── snag (the app)
                                     (toolbar click hides/shows the button; finds media)            │
                                                                   window, tray, Save windows ─────┤
                                                                     engine: files, yt-dlp ────────┘
```

Snag is one Go program (`cmd/snag`) with a web-view window ([Wails v3](https://v3.wails.io/)).
Browsers start a small helper, `snag-host`, for the extension; it relays to the app over a local
socket (a named pipe on Windows) and starts the app if it isn't running. [PROTOCOL.md](../PROTOCOL.md)
is what they say to each other. Installers and releases: [RELEASING.md](RELEASING.md).

```
cmd/snag/            the app's entry point: wires settings, the engine and the window together
cmd/snag-host/       the helper browsers start for the extension; relays to the app
cmd/fetch-tools/     downloads the pinned yt-dlp into bin/tools (`task tools`)
cmd/pack-extension/  packs the extension for the website: Chromium zip, Firefox .xpi
packaging/           installer files for Windows (NSIS), Linux (nfpm) and macOS
internal/app/        the desktop shell: application, window, tray, services for the UI
internal/service/     the app's background side: settings, engine, prober, the helper's server
internal/protocol/   the requests in PROTOCOL.md, answered with the engine and the prober
internal/ipc/        app <-> helper connection: JSON lines over a Unix socket / named pipe
internal/nativemsg/  the helper's relay: browser frames <-> the app, starting the app if needed
internal/nativehost/ registering the helper with each browser (manifests; registry on Windows)
internal/engine/     the download list: queue, pause/resume/remove, saving; runners plug in
internal/httpdl/     the runner for plain files: segmented, resumable HTTP downloads
internal/streamdl/   the runner for video sites and streams: runs yt-dlp, resumes, watch links
internal/probe/      what a page or stream offers: qualities and sizes, playlist sniffing
internal/playlist/   lists of many entries (playlists, channels, seasons, podcasts' feeds) and
                     turning the ones picked into downloads
internal/links/      a page's links, or links pasted, grouped for downloading many at once
internal/subscriptions/ lists followed: looking at them when due, downloading what is new
internal/checksum/   checksums as sites publish them, and checking files against them
internal/ytdlp/      yt-dlp's command line, output lines and JSON
internal/tools/      finding yt-dlp and ffmpeg (and fetching the pinned yt-dlp)
internal/feed/       Snag's signed feed: the latest release, and which yt-dlp to use
internal/update/     downloading a newer release's installer, checking it and installing it
internal/remote/     a paired phone's messages and pairing, for Snag Cloud's relay (not wired in yet)
internal/cookies/    browser cookies as a Cookie header or yt-dlp's cookie file
internal/naming/     file names: safe names, the server's name, real names vs tokens in addresses
internal/config/     settings with defaults and limits
internal/store/      JSON files written atomically, readable only by the user
internal/platform/   what differs per OS: where files live, stopping a program and its children,
                     starting at login, marking a finished file as downloaded
internal/watchlink/  a download's final name showing its growing file (hard link, else symlink)
internal/version/    the build's version
frontend/            the web UI (Svelte + TypeScript + Vite), embedded into the binary
```

The engine knows nothing about the UI or the OS. A runner gets a `Job` and reports through a
`Reporter`; everything unfinished stays in the job's work folder, which is what makes pause,
resume and restarts work. `internal/httpdl/httpdltest` is a file server for tests (ranges or not,
unknown length, dropped connections, slow); `internal/streamdl/streamtest` serves slow HLS
streams whose links can expire.

The app keeps `settings.json` in the usual config folder and `list.json`, a `work/` folder
(unfinished downloads, one folder each; a download saved to another drive keeps its parts in a
hidden `.snag` folder in its download folder instead, see `internal/engine/parts.go`), `tools/` (the copy of yt-dlp it keeps current), `feed.json` (the last
good feed) and `extension/` (the extension, ready to load unpacked) in the usual data folder. `SNAG_HOME=DIR`
keeps all of it, and the socket, in one folder instead: `SNAG_HOME=/tmp/snag-try bin/snag` tries
the app without touching your real list.

### The extension (extension/)

TypeScript in `src/`, bundled by esbuild (`npm run build`, or `task extension:build`) into
`build/`: one script each for the background and the pages, plus icons copied from
`packaging/icons`. The app embeds `build/` with `manifests/` (Manifest V3 for Chromium, V2 for
Firefox). `manifests/chromium.key.txt` is a public key that keeps the Chromium extension ID
stable; the helper's registration names that ID.

- `shared/`: the messages (`protocol.ts`; what Snag sends is in `wire.gen.ts`, made from the Go
  types by a test in `internal/protocol`: `go test ./internal/protocol -run TypeScript -update`),
  formatting, element helpers. Nothing is ever set as HTML.
- `media/`: plain logic, unit-tested: `detect.ts` tells media from everything else a page loads,
  `offers.ts` turns what was found into one list of options, `title.ts` names what the page plays.
- `background/`: finds media (network listener, the page's resource list, sniffing), talks to
  the app, catches browser downloads, the context menu, and the toolbar hide/show toggle.
- `content/`: the button and card on videos (`placement.ts` is their positioning, unit-tested).
  `card.ts` puts the card together from its parts: `page-offers.ts` (what the page offers),
  `clip-mode.ts` with `trim.ts` (the trim bar for clips), `subtitles.ts` and `have.ts`
  (what Snag already has).

## Building

**Needs:** Go 1.25+, Node 22+, the Wails CLI, and [Task](https://taskfile.dev/installation/):

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.28
```

On Linux also GTK 4 and WebKitGTK 6 (Arch: `gtk4 webkitgtk-6.0`; Debian/Ubuntu:
`libgtk-4-dev libwebkitgtk-6.0-dev`). Windows needs WebView2 (built into Windows 10 and 11);
macOS needs the Xcode command line tools.

```sh
task setup   # once: the frontend's and the extension's packages, and the Go modules
task tools   # once: the pinned yt-dlp, for streams and their tests
task dev     # run with a live-reloading UI (restart for Go changes)
task build   # release binary in bin/
task build:windows  # bin/snag.exe from any OS (CI also uploads each OS's build)
task install:local  # Linux: install for yourself in ~/.local/lib/snag, link ~/.local/bin/snag, browsers included
task test    # Go, window and extension tests
task lint    # golangci-lint, ESLint, Prettier, svelte-check, tsc
task package:linux  # and package:windows, package:macos, package:extension: see RELEASING.md
```

### The window (frontend/)

```
src/App.svelte              layout and keyboard shortcuts
src/lib/api/client.ts       the Go side: the generated bindings and events, behind one Api
src/lib/state/              Snag (the window's state and every action), the list, menus
src/lib/components/         Sidebar, Toolbar, AddBar, DownloadList/Row, ContextMenu,
                            SettingsSheet, ConnectSheet, and small parts (IconButton, Toggle, …)
src/SaveWindow.svelte       the Save window each waiting download gets
src/lib/format.ts           sizes, times, categories, the line under each download
src/test/                   a fake Go side (FakeApi), and tests that drive the whole window
```

The window gets its `Api` from `main.ts`; tests and the preview hand it a `FakeApi` instead, which
keeps downloads in memory, records calls and can make them fail or wait. Every action marks itself
busy until Go answers, so its button spins; failures become a toast.

`npm --prefix frontend run dev`, then open `http://localhost:9245/preview.html`, shows the window
with sample downloads and no Go side, for working on its looks: `?settings`, `?menu` and
`?connect` open those, `?save` shows a Save window, and `?save&taken&asked` one whose name is
taken.

## Tests

`task test` runs the Go tests, the window's (Vitest and Testing Library against the `FakeApi`) and
the extension's unit tests (`npm --prefix extension test`: finding media, the list of options,
names, placement).

The stream tests run the real yt-dlp: `task tools` fetches the pinned version into `bin/tools`,
and `task test` points the tests at it (they skip without it). The test that finishes a real
stream also needs ffmpeg on the PATH.

`internal/e2e` runs the extension in a headless Chromium-family browser against the app's
background side: it builds `snag-host`, registers it in a throwaway profile only, and drives the
extension over DevTools: its messages, and the Download button on a video (hovering it, opening
the card), which DevTools sees inside its closed shadow root. It needs a browser on the PATH
(`chromium`, `vivaldi`) or `$SNAG_BROWSER`, and skips without one (CI requires it). Branded
Chrome no longer loads extensions from the command line; use Chromium or Chrome for Testing
(`npx @puppeteer/browsers install chrome@stable`), as CI does.

The window can also be checked by eye without Go: `npm --prefix frontend run dev` and the preview
page (above).

## Things worth knowing

**Browsers**
- Reloading an unpacked extension doesn't update open tabs; the background script injects the new
  content scripts into them, and old copies remove themselves. Some Chromium browsers also cache the
  service worker until the extension is reloaded.
- Some browsers start native hosts with `LD_PRELOAD` set (Vivaldi preloads its own libffmpeg);
  `snag-host` drops it before starting the app, or the system ffmpeg breaks.
- Never hook the page's `fetch`/`XMLHttpRequest`: some players detect it and refuse to play.
  Disguised playlists are found from outside the page instead.
- In Chromium, `byExtensionId` is missing from `downloads.onCreated` for the extension's own
  downloads; the `handedBack` set keeps a handed-back download from being caught again.
- When a download is caught, the browser learns its real name a moment later; wait for it rather
  than fall back to the address, which on file hosts is a token.
- Headless test browsers: tabs made with `/json/new` may never load (navigate an existing tab),
  page-initiated downloads may not start, and media that can't play may never be reported to
  the extension (the card then names it from its address).

**Downloads**
- yt-dlp can ignore Ctrl-C while it fetches fragments in parallel; stopping it escalates to a kill
  after a grace period, for the whole process group (it runs ffmpeg).
- yt-dlp works entirely inside the download's work folder and Snag moves the result; letting
  yt-dlp move it onto the watch link could, across drives, copy through the link into the folder
  that is then deleted.
- Stream resume relies on yt-dlp's `.ytdl` + `.part` files under the same output name. Stream links
  often expire: adding the same video (page, title, quality) again gives the unfinished download
  the new link.
- yt-dlp's own size estimate for fragmented streams runs high with parallel fragments; Snag
  computes its own. Its progress also counts pieces fetched but not yet saved, which a pause
  drops, so progress may step back after a resume.
- On slow lines the line is the limit, not the connection count. Some speed-test servers reject
  `Mozilla/...` user agents; benchmark with `curl/8`.
- A download's parts stay on the drive it is saved to: a hard link (the watch link) can't cross
  drives, a symbolic link needs Developer Mode on Windows, and finishing across drives would copy
  the whole file. GitHub's Windows machines have the source on D: and temporary files on C:, so a
  test there downloads across drives for real.
- A file the user chose to replace is set aside (a hidden name in its folder) when the download
  starts, deleted when it finishes, and put back if it is removed.
- A finished file is marked as downloaded from the internet (`platform.MarkDownloaded`, through
  the engine's `Options.Mark`) before the download is reported done, so nothing offers to open it
  unmarked. Windows gets the Mark of the Web, a `Zone.Identifier` stream with the zone (3, the
  internet), the page and the address, which SmartScreen and Office's Protected View act on;
  macOS the quarantine attribute Gatekeeper acts on; Linux the freedesktop origin attributes file
  managers show (Linux has no check on downloaded programs). A drive whose file system can't
  hold the mark (FAT, exFAT) is skipped quietly; any other failure is logged, and the download is
  still done.
