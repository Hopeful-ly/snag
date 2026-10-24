import { describe, expect, it } from "vitest";
import {
  captionsOf,
  markNeedsPlayer,
  groups,
  options,
  qualityName,
  type StreamProbes,
} from "../src/media/offers";
import type { Choice, Media, PlaylistInfo } from "../src/shared/protocol";

const captionFile = (url: string, filename: string): Media => ({
  key: url,
  url,
  media: "subtitles",
  kind: "http",
  mime: "application/x-subrip",
  size: 8000,
  time: 0,
  frame: 0,
  filename,
});
const stream = (url: string, info?: PlaylistInfo): Media => ({
  key: url,
  url,
  media: "hls",
  kind: "ytdlp",
  mime: "",
  size: 0,
  time: 0,
  frame: 0,
  info,
});
const trailer: Media = {
  key: "t",
  url: "https://cdn.test/trailer.mp4",
  media: "video",
  kind: "http",
  mime: "video/mp4",
  size: 110e6,
  time: 0,
  frame: 0,
};
const choice = (label: string, size = 1): Choice => ({ label, detail: "MP4", format: "x", size });
const probed = (choices: Choice[], variants: string[]) => ({
  loading: false,
  result: { supported: true, choices, variants },
});

describe("what a page offers", () => {
  it("keeps a single-quality stream, which lists only itself", () => {
    const single = "https://cdn.test/vd/abc/index-s1080p-v1-a1.m3u8?t=1";
    const out = groups([stream(single), trailer], {
      [single]: probed([choice("1080p")], [single]),
    });
    expect(out.some((g) => g.rows.some((r) => r.item.url === single))).toBe(true);
  });

  it("folds a master's variant into it", () => {
    const master = "https://cdn.test/master.m3u8";
    const v720 = "https://cdn.test/v720.m3u8";
    const out = groups([stream(master), stream(v720)], {
      [master]: probed([choice("720p"), choice("360p")], [v720, "https://cdn.test/v360.m3u8"]),
      [v720]: probed([choice("720p")], [v720]),
    });
    expect(out).toHaveLength(1);
    expect(out[0]!.rows).toHaveLength(2);
  });

  it("leaves out subtitles and short ads next to a full-length stream", () => {
    const out = groups(
      [
        stream("https://cdn.test/subs.m3u8", { type: "subtitles" }),
        stream("https://cdn.test/ad.m3u8", { type: "media", duration: 30 }),
        stream("https://cdn.test/movie.m3u8", { type: "media", duration: 2700 }),
      ],
      {},
    );
    expect(out.map((g) => g.rows[0]!.item.url)).toEqual(["https://cdn.test/movie.m3u8"]);
  });

  const q = (h: number) => `https://cdn.test/vd/x/index-s${h}p-v1-a1.m3u8`;
  const heights = [480, 720, 2160, 1080];
  const probes: StreamProbes = {};
  for (const h of heights)
    probes[q(h)] = probed([{ ...choice(h + "p", h * 1e6), detail: "MP4 · 54 min" }], [q(h)]);

  it("lists every quality once, highest first, with files apart", () => {
    const list = options(null, heights.map((h) => stream(q(h))).concat([trailer]), probes);
    expect(list.video.map((o) => o.label)).toEqual(["2160p", "1080p", "720p", "480p"]);
    expect(list.video.map((o) => o.name)).toEqual(["4K", "Full HD", "HD", "SD"]);
    expect(list.video[0]!.best).toBe(true);
    expect(list.video[1]!.best).toBeUndefined();
    expect(list.video[0]!.size).toBe(2160e6);
    expect(list.files.map((f) => f.item.url)).toEqual([trailer.url]);
  });

  it("sorts streams not yet probed by the resolution in their address", () => {
    const pending = options(null, [stream(q(720)), stream(q(1080))], {});
    expect(pending.video.map((o) => o.label)).toEqual(["1080p", "720p"]);
  });

  it("lists the same resolution from the site and a stream once", () => {
    const site = {
      supported: true,
      url: "https://site/watch",
      title: "T",
      choices: [choice("1080p", 5), choice("720p", 3)],
    };
    const both = options(site, [stream(q(1080))], { [q(1080)]: probes[q(1080)]! });
    expect(both.video.filter((o) => o.height === 1080)).toHaveLength(1);
  });

  it("names qualities", () => {
    expect([2160, 1440, 1080, 720, 480, 0].map(qualityName)).toEqual([
      "4K",
      "QHD",
      "Full HD",
      "HD",
      "SD",
      "",
    ]);
  });

  it("keeps one chip per language and names it cleanly", () => {
    const extra = [
      {
        code: "und",
        name: "Spider-Man.Brand.New.Day.2026.1080p.WEBRip.x264.srt",
        url: "https://x/1.srt",
      },
      { code: "und", name: "Albanian (Spider)", url: "https://x/2.srt" },
      { code: "und", name: "Albanian (14054648)", url: "https://x/3.srt" },
      { code: "und", name: "Captions", text: "1\n00:00:01,000 --> 00:00:02,000\nHi\n" },
      { code: "en", name: "English" },
    ];
    const got = captionsOf({ supported: false }, {}, [], extra);
    expect(got.map((s) => s.name)).toEqual(["English", "Albanian"]);
  });

  it("leaves out a caption file it could not name", () => {
    const got = captionsOf(
      { supported: false },
      {},
      [],
      [
        { code: "und", name: "Captions", url: "https://x/subs.vtt" },
        { code: "en", name: "English", url: "https://x/en.vtt" },
      ],
    );
    expect(got.map((s) => s.name)).toEqual(["English"]);
  });

  it("uses the English file the player is showing, not an earlier one", () => {
    const got = captionsOf(
      { supported: false },
      {},
      [],
      [
        { code: "en", name: "English", url: "https://x/old.srt", text: "old" },
        { code: "en", name: "English", url: "https://x/new.srt", text: "new", showing: true },
      ],
    );
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ url: "https://x/new.srt", text: "new" });
  });

  it("uses the later English file when the player swapped without a showing flag", () => {
    const got = captionsOf(
      { supported: false },
      {},
      [],
      [
        { code: "en", name: "English", url: "https://x/old.srt", text: "old" },
        { code: "en", name: "English", url: "https://x/new.srt", text: "new" },
      ],
    );
    expect(got[0]).toMatchObject({ url: "https://x/new.srt", text: "new" });
  });

  it("takes captions from a file the player fetched", () => {
    const file = captionFile("https://cdn.test/subs/movie.en.srt", "movie.en.srt");
    const got = captionsOf({ supported: false }, {}, [file]);
    expect(got).toEqual([{ code: "en", name: "English", url: file.url }]);
  });

  it("uses the later caption file the player fetched for that language", () => {
    const older = captionFile("https://cdn.test/old.en.srt", "old.en.srt");
    const newer = captionFile("https://cdn.test/new.en.srt", "new.en.srt");
    const got = captionsOf({ supported: false }, {}, [older, newer]);
    expect(got).toEqual([{ code: "en", name: "English", url: newer.url }]);
  });

  it("takes captions from the stream when the page itself has none", () => {
    const streamUrl = "https://cdn.test/master.m3u8";
    const got = captionsOf(
      { supported: false },
      {
        [streamUrl]: {
          loading: false,
          result: {
            supported: true,
            subtitles: [
              { code: "en", name: "English" },
              { code: "es", name: "Español" },
            ],
          },
        },
      },
    );
    expect(got.map((s) => s.code)).toEqual(["en", "es"]);
  });
});

describe("markNeedsPlayer", () => {
  const captions = [
    { code: "en", name: "English", url: "https://www.youtube.com/api/timedtext?v=abc&lang=en" },
    { code: "es", name: "Spanish" },
    { code: "fr", name: "French", text: "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n" },
  ];

  it("marks YouTube captions the player hasn't loaded", () => {
    const got = markNeedsPlayer(captions, "https://www.youtube.com/watch?v=abc");
    expect(got.map((c) => [c.code, !!c.needsPlayer])).toEqual([
      ["en", false],
      ["es", true],
      ["fr", false],
    ]);
  });

  it("leaves other sites alone", () => {
    expect(markNeedsPlayer(captions, "https://vimeo.com/1")).toEqual(captions);
    expect(markNeedsPlayer(captions, "https://notyoutube.com/watch?v=abc")).toEqual(captions);
  });
});
