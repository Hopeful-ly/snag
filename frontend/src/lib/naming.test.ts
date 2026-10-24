import { describe, expect, it } from "vitest";
import { entryPreview, fileCredits, fillTemplate, joinFolder } from "./naming";

describe("joinFolder", () => {
  it("puts a list's folder on a place, and leaves a song's path as it is", () => {
    expect(joinFolder("/home/me/Videos", "Lanterns Season 1")).toBe(
      "/home/me/Videos/Lanterns Season 1",
    );
    expect(joinFolder("D:\\Videos", "Lanterns Season 1")).toBe("D:\\Videos\\Lanterns Season 1");
    expect(joinFolder("/home/me/Downloads", "")).toBe("/home/me/Downloads");
  });
});

describe("fillTemplate", () => {
  it("fills fields and drops empty ones with what joins them", () => {
    expect(fillTemplate("{artist} - {title}", { artist: "Daft Punk", title: "Get Lucky" })).toBe(
      "Daft Punk - Get Lucky",
    );
    expect(fillTemplate("{artist} - {title}", { title: "Get Lucky" })).toBe("Get Lucky");
    expect(fillTemplate("{n} - {title}", { n: "07", title: "Pilot" })).toBe("07 - Pilot");
    expect(fillTemplate("{title} ({year})", { title: "Get Lucky" })).toBe("Get Lucky");
    expect(fillTemplate("{show} S{season}E{episode} - {title}", { title: "Pilot" })).toBe("Pilot");
  });
});

describe("fileCredits", () => {
  it("names the album artist and keeps a feat tag the title already has", () => {
    expect(
      fileCredits(
        "Daft Punk, Pharrell Williams, Nile Rodgers",
        "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
        "Daft Punk",
      ),
    ).toEqual({
      artist: "Daft Punk",
      title: "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
    });
  });

  it("adds a feat tag when the title does not name the other artists", () => {
    expect(
      fileCredits("Daft Punk, Pharrell Williams, Nile Rodgers", "Get Lucky", "Daft Punk"),
    ).toEqual({
      artist: "Daft Punk",
      title: "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
    });
    expect(fileCredits("A, B, C, D", "Song", "A")).toEqual({
      artist: "A",
      title: "Song (feat. B, C and D)",
    });
  });

  it("leaves a collaboration, a list with no album artist, and Various Artists whole", () => {
    expect(fileCredits("A, B", "Team", "B, A")).toEqual({ artist: "A, B", title: "Team" });
    expect(fileCredits("Daft Punk, Pharrell Williams", "Get Lucky", "")).toEqual({
      artist: "Daft Punk, Pharrell Williams",
      title: "Get Lucky",
    });
    expect(fileCredits("Daft Punk, Pharrell Williams", "Song", "Various Artists")).toEqual({
      artist: "Daft Punk, Pharrell Williams",
      title: "Song",
    });
  });
});

describe("entryPreview", () => {
  it("names an episode from its tags and keeps the list's title for the old name", () => {
    const entry = {
      url: "https://v.example/1",
      title: "S1 E1 · Pilot",
      tags: { title: "Pilot", show: "Breaking Bad", season: 1, episode: 1 },
    };
    expect(entryPreview("{show} S{season}E{episode} - {title}", entry, 0, 62, "Breaking Bad")).toBe(
      "Breaking Bad S01E01 - Pilot",
    );
  });

  it("previews a song as its album artist, with who it features in the title", () => {
    const entry = {
      url: "https://open.spotify.com/track/1",
      title:
        "Daft Punk, Pharrell Williams, Nile Rodgers - Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
      tags: {
        title: "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
        artist: "Daft Punk, Pharrell Williams, Nile Rodgers",
        album_artist: "Daft Punk",
      },
    };
    expect(entryPreview("{artist} - {title}", entry, 0, 12, "Liked Songs")).toBe(
      "Daft Punk - Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
    );
  });
});
