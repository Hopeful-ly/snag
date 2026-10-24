import { describe, expect, it } from "vitest";
import { appleListOf, isAppleList, isAppleMusic } from "../src/media/apple";

describe("Apple Music pages", () => {
  it("names an album, a song in it, a playlist and an artist", () => {
    expect(appleListOf("https://music.apple.com/us/album/live/1545371283")).toBe("album");
    expect(appleListOf("https://music.apple.com/us/album/bitab-live/1545371283?i=1545371999")).toBe(
      "song",
    );
    expect(appleListOf("https://music.apple.com/us/song/bitab-live/1545371999")).toBe("song");
    expect(
      appleListOf(
        "https://music.apple.com/us/playlist/todays-hits/pl.f4d106fed2bd41149aaacabb233eb5eb",
      ),
    ).toBe("playlist");
    expect(appleListOf("https://music.apple.com/us/artist/mehrad-hidden/632147450")).toBe("artist");
    expect(isAppleList("https://music.apple.com/us/album/live/1545371283")).toBe(true);
    expect(isAppleMusic("https://music.apple.com/us/")).toBe(true);
    expect(appleListOf("https://music.apple.com/us/")).toBeNull();
    expect(appleListOf("https://music.apple.com/us/library/playlist/p.abc")).toBeNull();
    expect(isAppleList("https://open.spotify.com/album/4m2880jivSbbyEGAKfITCa")).toBe(false);
    expect(isAppleMusic("not a link")).toBe(false);
  });
});
