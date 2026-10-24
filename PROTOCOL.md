# Snag protocol

The app listens for its browser helper on a local socket only the user can open, `$SNAG_SOCKET`
when set:

| | |
|---|---|
| Linux | `$XDG_RUNTIME_DIR/snag/app.sock` |
| macOS | `~/Library/Application Support/Snag/app.sock` |
| Windows | named pipe `\\.\pipe\snag-<user>` |

Messages are JSON objects, one per line, both ways. The extension's types for what the app sends
(`extension/src/shared/wire.gen.ts`) are made from the Go ones, and a test fails when they differ.

## Requests

Every request has a `type` and may carry a `rid`; the answer is
`{"type": "reply", "rid": <same>, "ok": true, ...}` or `{"type": "reply", "rid": ..., "ok": false, "error": "..."}`.

| type | fields | answer |
|---|---|---|
| `ping` | | `version`, `ytdlp` (bool) |
| `list` | | `downloads` (newest first) |
| `subscribe` | | `downloads`; afterwards this connection also gets the events below |
| `add` | `url` (http/https), optional `kind` (`http` \| `ytdlp`, guessed if missing), `page`, `title`, `label`, `filename`, `directory`, `format`, `sort`, `audio` (`mp3` extracts audio), `subs` (caption language code), `sub_auto` (automatic captions), `sub_url` (a caption file the page loaded; fetched instead of asking the site), `sub_text` (cues already on the video, as `.srt`), `sidecar` (true writes a `.srt` next to the video instead of muxing), `clip_from`, `clip_to` (seconds: only that part of the video, cut exactly by yt-dlp whatever the address is, saved as H.264 MP4 named `<name> (m.ss-m.ss)`; captions are cut to match), `size` (estimate, shown until known), `headers` {name: value}, `cookies` [browser cookie objects], `checksum` (a checksum the page shows for the file: a bare hash, `sha256:<hex>`, `SHA-256: <hex>` or a checksum file's line), `checksum_files` (addresses of checksum files the page links to, which may list it), `start` (false adds it paused), `ask` (false skips the Save dialog) | `download`, `duplicate` (true: it was already there, unfinished; that one is returned, resumed if paused) |
| `have` | `page`, `title`, `items` `[{url, label, filename, format, audio, clip_from, clip_to}]` | `have`: one entry per item, the matching download or `null`. Same address and quality, same page and quality, or the same episode at that quality. A finished download whose file is gone is not a match, nor is a clip for the whole video or another clip. |
| `confirm` | `id` of a `pending` download, `directory`, optional `filename`, `start` (false: paused) | the Save dialog's answer; the folder joins `recent_dirs` |
| `sniff` | `url`, optional `headers`, `cookies` | `kind` (`hls` \| `dash`, or null when it isn't a playlist) and `text`: whether an address with a vague type is a stream's playlist |
| `probe` | `url` (a page, or an .m3u8/.mpd stream), optional `headers`, `cookies` | `probe`: `{supported, title, thumbnail, duration, site, url, choices: [{label, detail, format, sort, audio, size}], subtitles: [{code, name, auto}], variants: [playlist urls inside a stream], playlist: {title, count}}` or `{supported: false, reason}`. `playlist` is there when the page is in, or is, a list of two or more videos or tracks. Pages get one choice per height and frame rate; streams one per variant. Captions: uploader tracks, then automatic ones only for the video's language and English. |
| `playlist` | `url` (a page), optional `title` (the page's), `tmdb` (a copy of TMDB's API the page loaded its show from, without `/3`), `spotify` (on Spotify: `{token, client_token, app_version, user_agent, hashes: {query: hash}}`, how the open page asked Spotify's web player API, for reading all of a list; only used to read it, never kept), `open` (false: only say), `subscribe` (true: the user asked to follow the list, so its window opens ready to: nothing ticked, Keep downloading new ones on), `feeds` (the RSS and Atom feeds the page links to with `<link rel="alternate">`; the first three are read, and one counts only when it is a podcast's), `feeds_only` (true: look only in `feeds`, as the page itself was looked at already), `headers`, `cookies` | `playlist`: `{id, title, count}` for the list the page is in, or is (a playlist, channel, season or album, up to 1000 entries; a podcast's episodes, from an address that looks like a feed or a feed the page links to; a streaming site's show, its episodes from `tmdb` or from TMDB with the user's key; a Spotify playlist, album or song, whose songs are found on YouTube Music as they download: all of them with `spotify`, else the first 100), which Snag keeps for an hour and opens its Download playlist window for unless `open` is false; null for a page of one video |
| `links` | `page`, `title`, `source` (`page` or `selection`), `links` `[{url, text, name}]` (the page's links, images, videos and sounds, from every frame; `name` is an `<a download>` name), `headers`, `cookies` | `links`: `{id, count}`; Snag keeps the links an hour, tidied (absolute, http and https only, each once, without links to the page itself) and grouped (archives, programs, videos, audio, documents, images, video pages, web pages), and opens its Download links window for picking which to download. An error when there are none. |
| `resolved` | `token` (from a `resolve` event), `url` (the stream the page played), `headers`, `cookies` | |
| `resolve_failed` | `token`, `error` | |
| `kind` | `url` | `kind` |
| `pause`, `resume`, `retry`, `open`, `reveal` | `id` | |
| `remove` | `id`, optional `delete_file` | stops it if running; deletes partial files, and the finished file only with `delete_file` |
| `pause_all`, `resume_all`, `clear_done` | | |
| `config` | optional `set` {key: value} | `config` (`download_dir`, `connections`, `max_active`, `notify`, `ask`, `recent_dirs`) |
| `show` | optional `id` | brings up Snag's window; with `id`, that download's Save window |

`probe`, `sniff`, `playlist` and `links` run on their own threads, so a slow one doesn't hold up other requests on the same connection.

## Events (to subscribers)

- `{"type": "added", "download": {...}}`
- `{"type": "update", "download": {...}}`, at most 4 per second per download while it runs, always on a status change
- `{"type": "removed", "id": "..."}`
- `{"type": "config", "config": {...}}`
- `{"type": "resolve", "token": "...", "page": "..."}`, to one subscriber only (the one that asked something last): play the page (a streaming site's episode, when its download starts) and answer with `resolved` or `resolve_failed`. Snag gives up after a minute.

## A download

```
id, url, page, title, kind, label, status, path, name, size, done, speed, eta,
error, created, finished, connections, resumable, check
```

`status` is `pending` (waits for the Save dialog, when `ask` is on), `queued`, `downloading`,
`pausing`, `paused`, `done`, `error` or `removing`. `size` is 0 while unknown.
`speed` is bytes/s, `eta` seconds or null. `check` is null until a checksum for the file is
known, then `{algo, from, result}`: `algo` `md5` | `sha1` | `sha256` | `sha384` | `sha512`, `from`
`page` | `server` | `github` | `file` | `you`, and `result` `""` until the user asks Snag to check
the finished file, then `checking`, `match`, `mismatch` or `failed`. Snag finds checksums by
itself (the page's, the server's headers, GitHub's release records, checksum files next to the
file) but only reads a file to check it when the user asks.

## The native messaging host

`snag-host` (`cmd/snag-host`) sits between the browser extension and the app. It forwards only
the request types it allows (no `config`, no `confirm`), relays every line from the app to the extension, starts the app
(`snag --background`) if nothing listens, and sends `{"type": "ready"}` once connected (`{"type": "error"}` if it
couldn't start the app).
