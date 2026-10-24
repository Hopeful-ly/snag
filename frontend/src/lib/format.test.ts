import { describe, expect, it } from "vitest";
import type { Download } from "./api/client";
import {
  ago,
  categoryOf,
  checkText,
  duration,
  host,
  looksLikeChecksum,
  meta,
  subscriptionKind,
  subscriptionSource,
  subscriptionStatus,
  until,
  percent,
  size,
} from "./format";

export function download(fields: Partial<Download>): Download {
  return {
    id: "1",
    url: "https://example.com/file.zip",
    page: "",
    title: "",
    kind: "http",
    label: "",
    status: "queued",
    name: "file.zip",
    path: "",
    directory: "/dl",
    error: "",
    note: "",
    size: 0,
    estimated: false,
    done: 0,
    speed: 0,
    eta: null,
    connections: 0,
    resumable: true,
    created: 0,
    finished: 0,
    watch: "",
    watchIn: null,
    queue: "main",
    position: 0,
    subscription: "",
    where: false,
    check: null,
    stage: "",
    stageDone: 0,
    stageOf: 0,
    pieces: 0,
    piecesDone: 0,
    heard: 0,
    ...fields,
  };
}

describe("size", () => {
  it.each([
    [0, ""],
    [512, "512 B"],
    [1536, "2 KB"],
    [5.2 * 2 ** 20, "5.2 MB"],
    [25 * 2 ** 20, "25 MB"],
    [2.75 * 2 ** 30, "2.8 GB"],
  ])("%d bytes is %j", (bytes, text) => expect(size(bytes)).toBe(text));
});

describe("duration and ago", () => {
  it("writes time left", () => {
    expect(duration(42)).toBe("42 s");
    expect(duration(75)).toBe("1 min 15 s");
    expect(duration(3 * 3600 + 120)).toBe("3 h 2 min");
    expect(duration(null)).toBe("");
  });

  it("writes how long ago", () => {
    const now = Date.UTC(2026, 9, 7, 12);
    expect(ago(now - 20_000, now)).toBe("just now");
    expect(ago(now - 5 * 60_000, now)).toBe("5 min ago");
    expect(ago(now - 3 * 3600_000, now)).toBe("3 h ago");
    expect(ago(now - 2 * 86400_000, now)).toBe("2 d ago");
    expect(ago(0, now)).toBe("");
  });
});

describe("categoryOf", () => {
  it.each([
    [{ name: "Clip.mp4" }, "video"],
    [{ name: "A film", kind: "stream" }, "video"],
    [{ name: "A film", kind: "stream", label: "Audio MP3" }, "music"],
    [{ name: "song.flac" }, "music"],
    [{ name: "ubuntu.iso" }, "archives"],
    [{ name: "setup.exe" }, "programs"],
    [{ name: "report.pdf" }, "documents"],
    [{ name: "data.bin2" }, "other"],
  ])("%j is %s", (fields, id) => expect(categoryOf(download(fields)).id).toBe(id));
});

describe("meta", () => {
  it("says how a running download is doing", () => {
    const d = download({
      status: "downloading",
      size: 10 * 2 ** 20,
      done: 2.5 * 2 ** 20,
      speed: 2 ** 20,
      eta: 8,
    });
    expect(meta(d)).toBe("2.5 MB of 10 MB  ·  1.0 MB/s  ·  8 s left");
    expect(meta({ ...d, estimated: true })).toContain("of ~10 MB");
    expect(meta(download({ status: "downloading" }))).toBe("Starting…");
    expect(
      meta(download({ status: "downloading", note: "Trying again in 5 seconds (1 of 3)" })),
    ).toBe("Trying again in 5 seconds (1 of 3)");
  });

  it("says what a finished download was", () => {
    const now = Date.UTC(2026, 9, 7, 12);
    const d = download({
      status: "done",
      size: 2 ** 20,
      label: "1080p",
      page: "https://www.example.com/watch",
      finished: now - 120_000,
    });
    expect(meta(d, now)).toBe("1.0 MB  ·  1080p  ·  example.com  ·  2 min ago");
    expect(meta({ ...d, note: "Saved without subtitles: the site refused them" }, now)).toBe(
      "Saved without subtitles: the site refused them",
    );
  });

  it.each([
    [{ status: "error", error: "server said 404 Not Found" }, "server said 404 Not Found"],
    [
      { status: "error", where: true, error: "couldn't find “Bitab” on YouTube Music: no match" },
      "Try again on YouTube Music, or somewhere else",
    ],
    [
      { status: "pending", label: "720p", size: 2 ** 20, estimated: true },
      "Choose where to save it  ·  720p  ·  ~1.0 MB",
    ],
    [{ status: "queued", size: 2 ** 20 }, "Waiting for a free slot  ·  0 B of 1.0 MB"],
    [{ status: "paused", done: 2 ** 20 }, "Paused  ·  1.0 MB"],
    [{ status: "pausing" }, "Pausing…"],
    [{ status: "removing" }, "Removing…"],
  ])("%j", (fields, text) => expect(meta(download(fields as Partial<Download>))).toBe(text));
});

describe("percent and host", () => {
  it("works out what it can", () => {
    expect(percent(download({ size: 200, done: 90 }))).toBe(45);
    expect(percent(download({ size: 0, done: 90 }))).toBe(-1);
    expect(host("https://www.example.com:8080/a")).toBe("example.com");
    expect(host("not a link")).toBe("");
  });
});

describe("checksums", () => {
  const found = {
    algo: "SHA-256",
    weak: false,
    from: "page",
    result: "",
    want: "ab".repeat(32),
    got: "",
    error: "",
    read: 0,
  };

  it("says where a check stands", () => {
    expect(checkText(found)).toBe("SHA-256 found on the page");
    expect(checkText({ ...found, from: "github" })).toBe("SHA-256 found on GitHub");
    expect(checkText({ ...found, result: "checking", read: 40 }, 100)).toBe(
      "Checking SHA-256… 40%",
    );
    expect(checkText({ ...found, result: "match" })).toBe("SHA-256 matches (from the page)");
    expect(checkText({ ...found, result: "match", from: "you" })).toBe(
      "SHA-256 matches (the one you pasted)",
    );
    expect(checkText({ ...found, result: "mismatch" })).toBe(
      "Doesn't match its SHA-256: the file may be damaged or changed",
    );
    expect(checkText({ ...found, result: "failed", error: "permission denied" })).toBe(
      "Couldn't check it: permission denied",
    );
  });

  it("puts it under a finished file's name", () => {
    const d = download({ status: "done", size: 2048, check: { ...found, result: "match" } });
    expect(meta(d)).toBe("2 KB  ·  SHA-256 matches (from the page)");
    const looking = download({
      status: "done",
      size: 2048,
      page: "https://example.com",
      check: { ...found, want: "", result: "looking" },
    });
    expect(meta(looking, 0)).toBe("2 KB  ·  example.com");
  });

  it("knows a checksum when it sees one", () => {
    for (const text of [
      "ab".repeat(32),
      "sha256:" + "ab".repeat(32),
      "SHA-256: " + "AB".repeat(32),
      "ab".repeat(16) + "  file.iso",
      " " + "ab".repeat(64) + "\n",
    ])
      expect(looksLikeChecksum(text), text).toBe(true);
    for (const text of ["", "hello", "ab".repeat(31), "https://example.com/" + "ab".repeat(32)])
      expect(looksLikeChecksum(text), text).toBe(false);
  });
});

describe("subscriptions", () => {
  const now = Date.UTC(2026, 9, 23, 12);
  const hour = 3_600_000;

  it("says when the next look is", () => {
    expect(until(now + 30_000, now)).toBe("soon");
    expect(until(now - hour, now)).toBe("soon");
    expect(until(now + 20 * 60_000, now)).toBe("in 20 min");
    expect(until(now + 4 * hour, now)).toBe("in 4 h");
    expect(until(now + 50 * hour, now)).toBe("in 2 d");
  });

  it("names what is followed", () => {
    const kind = (source: string, podcast = false) => subscriptionKind({ source, podcast });
    expect(kind("https://www.youtube.com/@lanterns/videos")).toBe("YouTube channel");
    expect(kind("https://www.youtube.com/playlist?list=PL1")).toBe("YouTube playlist");
    expect(kind("https://feeds.example.org/harbour.rss", true)).toBe("Podcast");
    expect(kind("https://open.spotify.com/playlist/37i9")).toBe("Spotify playlist");
    expect(kind("https://vimeo.com/channels/staffpicks")).toBe("vimeo.com");
    const source = (source: string, podcast = false) => subscriptionSource({ source, podcast });
    expect(source("https://www.youtube.com/@lanterns/videos")).toBe("YouTube channel");
    expect(source("https://feeds.example.org/harbour.rss", true)).toBe(
      "Podcast  ·  feeds.example.org",
    );
    expect(source("https://vimeo.com/channels/staffpicks")).toBe("vimeo.com");
  });

  it("says what a subscription did and will do", () => {
    const s = {
      checking: false,
      checked: now - 2 * hour,
      next: now + 4 * hour,
      paused: false,
      last: 2,
      added: 5,
    } as Parameters<typeof subscriptionStatus>[0];
    expect(subscriptionStatus(s, now)).toBe(
      "Looked 2 h ago, next in 4 h, 2 new last time, 5 downloaded",
    );
    expect(subscriptionStatus({ ...s, paused: true, last: 0, added: 0 }, now)).toBe(
      "Looked 2 h ago, paused",
    );
    expect(subscriptionStatus({ ...s, checked: 0, next: 0, last: 0, added: 0 }, now)).toBe(
      "Not looked at yet",
    );
    expect(subscriptionStatus({ ...s, checking: true }, now)).toBe("Looking for new ones now…");
  });
});
