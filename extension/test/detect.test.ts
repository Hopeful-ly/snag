import { describe, expect, it } from "vitest";
import {
  classify,
  captionForPage,
  classifyCaption,
  classifySeen,
  dispositionName,
  extensionOf,
  meaningfulName,
  mightBePlaylist,
  readPlaylist,
  type Response,
} from "../src/media/detect";

const response = (
  url: string,
  headers: Record<string, string> = {},
  more: Partial<Response> = {},
): Response => ({
  tabId: 1,
  statusCode: 200,
  url,
  type: "media",
  responseHeaders: Object.entries(headers).map(([name, value]) => ({ name, value })),
  ...more,
});

describe("finding media in responses", () => {
  it("knows playlists by type or address", () => {
    expect(classify(response("https://cdn.x/a/master.m3u8"))?.media).toBe("hls");
    expect(classify(response("https://cdn.x/get?f=m3u8"))?.media).toBe("hls");
    expect(
      classify(response("https://cdn.x/a", { "Content-Type": "application/vnd.apple.mpegurl" }))
        ?.media,
    ).toBe("hls");
    expect(classify(response("https://cdn.x/a/manifest.mpd"))?.media).toBe("dash");
  });

  it("takes video and audio files of a useful size, with their names", () => {
    const film = classify(
      response("https://cdn.x/v/9f8e7d6c5b4a.mp4", {
        "content-type": "video/mp4",
        "content-length": String(50e6),
        "content-disposition": "attachment; filename*=UTF-8''My%20Holiday.mp4",
      }),
    );
    expect(film).toEqual({
      media: "video",
      kind: "http",
      mime: "video/mp4",
      size: 50e6,
      filename: "My Holiday.mp4",
    });
    const song = classify(
      response("https://cdn.x/music/Morning_Song.mp3", {
        "content-type": "audio/mpeg",
        "content-range": "bytes 0-99/4000000",
      }),
    );
    expect(song).toMatchObject({ media: "audio", size: 4e6, filename: "Morning_Song.mp3" });
  });

  it("passes over pieces, small files, other sites' ads and failed requests", () => {
    const video = { "content-type": "video/mp4" };
    expect(classify(response("https://cdn.x/v/seg-12.m4s", video))).toBeNull();
    expect(classify(response("https://cdn.x/v/chunk_3.mp4", video))).toBeNull();
    expect(classify(response("https://cdn.x/v/a.mp4?range=0-1000", video))).toBeNull();
    expect(
      classify(response("https://cdn.x/v/ping.mp4", { ...video, "content-length": "5000" })),
    ).toBeNull();
    expect(classify(response("https://r3.googlevideo.com/videoplayback", video))).toBeNull();
    expect(classify(response("https://cdn.x/v/a.mp4", video, { statusCode: 404 }))).toBeNull();
    expect(classify(response("https://cdn.x/v/a.mp4", video, { tabId: -1 }))).toBeNull();
    expect(
      classify(response("https://cdn.x/page.html", { "content-type": "text/html" })),
    ).toBeNull();
  });

  it("reads what the page itself fetched", () => {
    expect(
      classifySeen({ url: "https://cdn.x/a/index.m3u8", initiator: "xmlhttprequest" })?.media,
    ).toBe("hls");
    expect(classifySeen({ url: "https://cdn.x/a/clip.webm", initiator: "video" })).toMatchObject({
      media: "video",
      filename: "clip.webm",
    });
    expect(classifySeen({ url: "https://cdn.x/a/clip.webm", initiator: "fetch" })).toBeNull();
  });

  it("looks inside small script-fetched responses of vague type", () => {
    const xhr = { type: "xmlhttprequest" };
    expect(
      mightBePlaylist(response("https://cdn.x/api/stream", { "content-type": "text/plain" }, xhr)),
    ).toBe(true);
    expect(
      mightBePlaylist(response("https://cdn.x/app.js", { "content-type": "text/plain" }, xhr)),
    ).toBe(false);
    expect(
      mightBePlaylist(
        response("https://cdn.x/api/data", { "content-type": "application/json" }, xhr),
      ),
    ).toBe(false);
    expect(mightBePlaylist(response("https://cdn.x/api/stream", {}, { type: "image" }))).toBe(
      false,
    );
  });
});

describe("names", () => {
  it("reads Content-Disposition", () => {
    expect(dispositionName('attachment; filename="report final.pdf"')).toBe("report final.pdf");
    expect(dispositionName("attachment; filename=a%20b.zip")).toBe("a b.zip");
    expect(dispositionName('attachment; filename="..\\..\\evil.exe"')).toBe("evil.exe");
    expect(dispositionName("inline")).toBe("");
  });

  it("takes real names from addresses, not tokens and placeholders", () => {
    expect(meaningfulName("https://x.io/files/Holiday%20Video.mp4")).toBe("Holiday Video.mp4");
    for (const url of [
      "https://x.io/master.m3u8",
      "https://x.io/index-v1.mp4",
      "https://x.io/1434659607842-pgv4ql.mp4",
      "https://x.io/7a1f3c9e2b4d.mp4",
      "https://x.io/KEOeplOG8iaPmzsqlKupxyzAB.mp4",
      "https://x.io/noextension",
    ]) {
      expect(meaningfulName(url), url).toBe("");
    }
  });

  it("finds a file's extension by address or type", () => {
    expect(extensionOf("https://x.io/a/Clip.WEBM")).toBe(".webm");
    expect(extensionOf("https://x.io/a/get", "audio/mpeg")).toBe(".mp3");
    expect(extensionOf("https://x.io/a/get", "text/plain")).toBe("");
  });
});

describe("caption files", () => {
  it("takes a complete .srt, not an HLS vtt segment", () => {
    const srt = classifyCaption(
      response("https://cdn.x/subs/movie.en.srt", {
        "content-type": "application/x-subrip",
        "content-length": "8000",
      }),
    );
    expect(srt).toMatchObject({ media: "subtitles", filename: "movie.en.srt" });
    expect(
      classifyCaption(
        response("https://cdn.x/hls/seg-12.vtt", {
          "content-type": "text/vtt",
          "content-length": "400",
        }),
      ),
    ).toBeNull();
    expect(
      classifyCaption(
        response("https://cdn.x/hls/seg-12.vtt", {
          "content-type": "text/vtt",
          "content-length": "8000",
        }),
      ),
    ).toBeNull();
  });

  it("takes YouTube's player captions, asking for .srt with the player's token", () => {
    const player =
      "https://www.youtube.com/api/timedtext?v=9OTND8qWR0k&caps=asr&kind=asr&lang=en&fmt=json3&pot=TOKEN&c=WEB";
    const found = classifyCaption(response(player, { "content-type": "application/json" }));
    expect(found).toMatchObject({ media: "subtitles", kind: "http", mime: "application/x-subrip" });
    const url = new URL(found!.url!);
    expect(url.searchParams.get("fmt")).toBe("srt");
    expect(url.searchParams.get("pot")).toBe("TOKEN");
    expect(url.searchParams.get("lang")).toBe("en");
    expect(
      classifyCaption(
        response("https://www.youtube.com/api/timedtext?type=list&v=x", {
          "content-type": "application/json",
        }),
      ),
    ).toBeNull();
  });

  it("keeps a YouTube caption only on the video it is for", () => {
    const cap = "https://www.youtube.com/api/timedtext?v=abc&lang=en&fmt=vtt";
    expect(captionForPage(cap, "https://www.youtube.com/watch?v=abc&t=10")).toBe(true);
    expect(captionForPage(cap, "https://www.youtube.com/watch?v=other")).toBe(false);
    expect(captionForPage(cap, "https://www.youtube.com/shorts/abc")).toBe(true);
    expect(captionForPage("https://cdn.x/subs/movie.en.srt", "https://x/watch?v=1")).toBe(true);
  });

  it("takes an OpenSubtitles download without a .srt in the path", () => {
    const got = classifyCaption(
      response("https://dl.opensubtitles.org/en/download/file/src-api/123", {
        "content-type": "application/octet-stream",
        "content-disposition": 'attachment; filename="Movie.en.srt"',
        "content-length": "40000",
      }),
    );
    expect(got).toMatchObject({ media: "subtitles", filename: "Movie.en.srt" });
  });

  it("does not treat leftover .vtt files as something to download", () => {
    expect(
      classifySeen({ url: "https://cdn.x/subs/en.vtt", initiator: "xmlhttprequest" }),
    ).toBeNull();
  });

  it("takes a captions API file with a language, not the JSON list or an HTML page", () => {
    expect(
      classifyCaption(
        response("https://yt.example/api/v1/captions/abc?label=English", {
          "content-type": "text/vtt",
          "content-length": "12000",
        }),
      ),
    ).toMatchObject({ media: "subtitles" });
    expect(
      classifyCaption(
        response("https://yt.example/api/v1/captions/abc?label=English", {
          "content-length": "12000",
        }),
      ),
    ).toMatchObject({ media: "subtitles" });
    expect(
      classifyCaption(
        response("https://yt.example/api/v1/captions/abc", {
          "content-type": "application/json",
          "content-length": "4000",
        }),
      ),
    ).toBeNull();
    expect(
      classifyCaption(
        response("https://yt.example/api/v1/captions/abc?label=English", {
          "content-type": "text/html",
          "content-length": "8000",
        }),
      ),
    ).toBeNull();
  });
});

describe("playlists", () => {
  it("tells a master from a stream, subtitles and audio", () => {
    const master =
      "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1,RESOLUTION=1280x720\nhi.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=2,RESOLUTION=1920x1080\nhd.m3u8\n";
    expect(readPlaylist("hls", master)).toEqual({ type: "master", variants: 2, height: 1080 });
    const vod = "#EXTM3U\n#EXTINF:4.5,\na.ts\n#EXTINF:5.5,\nb.ts\n#EXT-X-ENDLIST\n";
    expect(readPlaylist("hls", vod)).toEqual({ type: "media", duration: 10, live: false });
    expect(
      readPlaylist("hls", "#EXTM3U\n#EXTINF:6,\nsub1.vtt\n#EXTINF:6,\nsub2.vtt?x=1\n").type,
    ).toBe("subtitles");
    expect(readPlaylist("hls", "#EXTM3U\n#EXTINF:6,\na.aac\n").type).toBe("audio");
    expect(readPlaylist("hls", "#EXTM3U\n#EXTINF:6,\na.ts\n").live).toBe(true);
  });

  it("reads a DASH manifest's length", () => {
    expect(readPlaylist("dash", '<MPD mediaPresentationDuration="PT1H2M3.5S">')).toEqual({
      type: "dash",
      duration: 3723.5,
    });
  });
});
