import { describe, expect, it } from "vitest";
import { pickStream, tmdbCopyIn, tmdbCopyOf } from "../src/media/resolve";
import type { Media } from "../src/shared/protocol";

const media = (fields: Partial<Media>): Media => ({
  key: fields.url ?? "k",
  url: "https://cdn.example/x",
  media: "hls",
  kind: "ytdlp",
  mime: "",
  size: 0,
  time: 0,
  frame: 0,
  ...fields,
});

describe("finding what a page plays", () => {
  it("takes a stream with every quality first", () => {
    const ad = media({ url: "https://ads.example/a.m3u8", info: { type: "media", duration: 20 } });
    const master = media({ url: "https://cdn.example/master.m3u8", info: { type: "master" } });
    expect(pickStream([ad, master])?.url).toBe(master.url);
  });

  it("else the longest stream, as ads are short", () => {
    const ad = media({ url: "https://ads.example/a.m3u8", info: { type: "media", duration: 20 } });
    const episode = media({
      url: "https://cdn.example/1080.m3u8",
      info: { type: "media", duration: 3300 },
    });
    expect(pickStream([ad, episode])?.url).toBe(episode.url);
  });

  it("leaves out captions, sound alone and live streams", () => {
    expect(
      pickStream([
        media({ info: { type: "subtitles" } }),
        media({ info: { type: "audio" } }),
        media({ info: { type: "media", live: true } }),
      ]),
    ).toBeNull();
  });

  it("falls back to a video file, but not a preview", () => {
    const trailer = media({ media: "video", size: 3 * 1024 * 1024 });
    const film = media({
      url: "https://cdn.example/film.mp4",
      media: "video",
      size: 900 * 1024 * 1024,
    });
    expect(pickStream([trailer])).toBeNull();
    expect(pickStream([trailer, film])?.url).toBe(film.url);
  });
});

describe("noticing a copy of TMDB's API", () => {
  it("names the copy a show's request went to", () => {
    expect(tmdbCopyOf("https://api.tmdb-copy.example/3/tv/95350/season/1?language=en")).toBe(
      "https://api.tmdb-copy.example",
    );
    expect(tmdbCopyOf("https://api.example/tmdb/3/tv/95350")).toBe("https://api.example/tmdb");
  });

  it("keeps the key a page asks TMDB itself with", () => {
    expect(
      tmdbCopyOf("https://api.themoviedb.org/3/tv/1396/season/1?include_adult=false&api_key=84a"),
    ).toBe("https://api.themoviedb.org?api_key=84a");
  });

  it("picks the page's copy of what it loaded, a keyed one first", () => {
    expect(
      tmdbCopyIn([
        "https://streamer.example/app.js",
        "https://api.themoviedb.org/3/tv/1396?api_key=k",
        "https://api.themoviedb.org/3/tv/1396/season/1",
      ]),
    ).toBe("https://api.themoviedb.org?api_key=k");
    expect(tmdbCopyIn(["https://db.example/3/tv/1"])).toBe("https://db.example");
    expect(tmdbCopyIn(["https://example.com/"])).toBe("");
  });

  it("ignores everything else", () => {
    expect(tmdbCopyOf("https://db.example/3/movie/12")).toBeNull();
    expect(tmdbCopyOf("https://db.example/3/tv/95350/season/1/episode/2")).toBeNull();
    expect(tmdbCopyOf("not an address")).toBeNull();
  });
});
