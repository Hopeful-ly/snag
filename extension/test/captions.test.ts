import { describe, expect, it } from "vitest";
import { captionLang, catalogTracks, cuesToSRT, tracksOf } from "../src/media/captions";

describe("caption language from a file's name or address", () => {
  it("reads the language from a filename", () => {
    expect(captionLang("https://x/a", "Movie.en.srt")).toEqual({ code: "en", name: "English" });
    expect(captionLang("https://x/a", "arabic.srt")).toEqual({ code: "ar", name: "Arabic" });
    expect(
      captionLang("https://www.youtube.com/api/timedtext?v=x&lang=uk&tlang=en&kind=asr"),
    ).toEqual({ code: "en", name: "English" });
    expect(captionLang("https://x/a", "Albanian (Spider)")).toEqual({
      code: "sq",
      name: "Albanian",
    });
  });

  it("reads it from the path or query", () => {
    expect(captionLang("https://cdn.x/subtitles/bg.vtt")).toEqual({
      code: "bg",
      name: "Bulgarian",
    });
    expect(captionLang("https://cdn.x/get?lang=eng")).toEqual({ code: "en", name: "English" });
  });

  it("writes cues as SubRip", () => {
    const srt = cuesToSRT([
      { startTime: 1, endTime: 2.5, text: "Hello" },
      { startTime: 62.04, endTime: 63, text: "There" },
    ]);
    expect(srt).toContain("1\n00:00:01,000 --> 00:00:02,500\nHello");
    expect(srt).toContain("2\n00:01:02,040 --> 00:01:03,000\nThere");
  });

  it("falls back when nothing says", () => {
    expect(captionLang("https://cdn.x/download/file/123")).toEqual({
      code: "und",
      name: "Captions",
    });
  });
});

describe("captions on this video, not a search catalog", () => {
  it("tells a catalog dump from the languages this video ships", () => {
    expect(
      catalogTracks([
        { src: "https://dl.opensubtitles.org/1", label: "Albanian (Spider)" },
        { src: "https://dl.opensubtitles.org/2", label: "Croatian (KyoGo)" },
        { src: "https://dl.opensubtitles.org/3", label: "Movie.1080p.WEBRip.srt" },
      ]),
    ).toBe(true);
    expect(
      catalogTracks([
        { src: "https://cdn.site/en.vtt", label: "English" },
        { src: "https://cdn.site/es.vtt", label: "Spanish" },
      ]),
    ).toBe(false);
  });

  it("does not offer unused OpenSubtitles tracks", () => {
    const video = document.createElement("video");
    for (const [label, src] of [
      ["Albanian (Spider)", "https://dl.opensubtitles.org/1"],
      ["Croatian (KyoGo)", "https://dl.opensubtitles.org/2"],
      ["Movie.1080p.WEBRip.srt", "https://dl.opensubtitles.org/3"],
    ] as const) {
      const t = document.createElement("track");
      t.kind = "subtitles";
      t.label = label;
      t.src = src;
      video.append(t);
    }
    expect(tracksOf(video)).toEqual([]);
  });

  it("offers the two languages this video ships", () => {
    const video = document.createElement("video");
    video.append(
      track("en", "English", "https://cdn.site/en.vtt"),
      track("es", "Spanish", "https://cdn.site/es.vtt"),
    );
    expect(tracksOf(video).map((t) => t.name)).toEqual(["English", "Spanish"]);
  });
});

function track(lang: string, label: string, src: string): HTMLTrackElement {
  const t = document.createElement("track");
  t.kind = "subtitles";
  t.srclang = lang;
  t.label = label;
  t.src = src;
  return t;
}
