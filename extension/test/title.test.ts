import { describe, expect, it } from "vitest";
import { compose, numbersIn, numbersInUrl, structuredData, type Clues } from "../src/media/title";

const named = (clues: Partial<Clues>) =>
  compose({ url: "https://example.org/", host: "example.org", ...clues }).title;

const site = { url: "https://www.streamer.example/tv/95350/1/3", host: "www.streamer.example" };

describe("naming what a page plays", () => {
  it.each<[string, Partial<Clues>, string]>([
    [
      "site title and numbers in the address",
      { ...site, docTitle: "Lanterns | Streamer" },
      "Lanterns S01E03",
    ],
    [
      "the player names the episode",
      {
        ...site,
        docTitle: "Lanterns | Streamer",
        session: { title: "Lanterns S1E3 - In Brightest Day" },
      },
      "Lanterns S01E03 - In Brightest Day",
    ],
    [
      "a heading names the episode",
      {
        ...site,
        docTitle: "Lanterns | Streamer",
        headings: ["Lanterns", "Season 1", "E2 - Before", "E3 - In Brightest Day", "E4 - After"],
      },
      "Lanterns S01E03 - In Brightest Day",
    ],
    [
      "Watch … Online Free titles",
      {
        url: "https://watcher.example/watch/abc",
        host: "watcher.example",
        docTitle: "Watch Lanterns Season 1 Episode 3 Online Free - Watcher",
      },
      "Lanterns S01E03",
    ],
    [
      "the episode's name in the title",
      { url: "https://a.tv/x", host: "a.tv", docTitle: "Lanterns - S01E03 - The Ring | A" },
      "Lanterns S01E03 - The Ring",
    ],
    [
      "structured data",
      {
        ld: { series: "Lanterns", season: 1, episode: 3, episodeName: "The Ring" },
        docTitle: "Episode page",
      },
      "Lanterns S01E03 - The Ring",
    ],
    [
      "structured data with a generic episode name",
      {
        ld: { series: "Lanterns", season: 2, episode: 10, episodeName: "Episode 10" },
        docTitle: "x",
      },
      "Lanterns S02E10",
    ],
    ["1x03", { docTitle: "Lanterns 1x03 | Site", host: "site.com" }, "Lanterns S01E03"],
    [
      "numbers in an address slug",
      { url: "https://b.io/watch/lanterns-s01e03", host: "b.io", docTitle: "Lanterns | B" },
      "Lanterns S01E03",
    ],
    [
      "query parameters",
      { url: "https://c.cc/play?id=9&season=2&episode=5", host: "c.cc", docTitle: "Lanterns – C" },
      "Lanterns S02E05",
    ],
    [
      "a film with its year",
      {
        ld: { series: "Spider-Man: Brand New Day", year: "2026" },
        docTitle: "Spider-Man: Brand New Day | Streamer",
        host: "streamer.example",
      },
      "Spider-Man Brand New Day (2026)",
    ],
    [
      "a film title with a colon stays whole",
      {
        docTitle: "Spider-Man: Brand New Day | Streamer",
        host: "streamer.example",
        url: "https://streamer.example/movie/969681",
      },
      "Spider-Man Brand New Day",
    ],
    [
      "a number in a film's title is not an episode",
      {
        docTitle: "Blade Runner 2049 | Streamer",
        host: "streamer.example",
        url: "https://streamer.example/movie/335984",
      },
      "Blade Runner 2049",
    ],
    [
      "YouTube",
      {
        docTitle: "Me at the zoo - YouTube",
        host: "www.youtube.com",
        url: "https://www.youtube.com/watch?v=jNQXAC9IVRw",
      },
      "Me at the zoo",
    ],
    [
      "og:title over a padded tab title",
      {
        ogTitle: "Lanterns",
        docTitle: "Watch Lanterns (2026) Online Free HD - Streamer",
        host: "streamer.example",
        url: "https://streamer.example/tv/95350/1/3",
      },
      "Lanterns S01E03",
    ],
  ])("%s", (_, clues, want) => {
    expect(named(clues)).toBe(want);
  });

  it("finds season and episode in text and addresses", () => {
    expect(numbersIn("Season 2 - Episode 11")).toEqual({ season: 2, episode: 11 });
    expect(numbersIn("Blade Runner 2049")).toBeNull();
    expect(numbersInUrl("https://x.io/show/season/3/episode/7")).toEqual({ season: 3, episode: 7 });
    expect(numbersInUrl("not an address")).toBeNull();
  });

  it("reads schema.org data, including a @graph, and ignores broken data", () => {
    document.head.innerHTML = `
      <script type="application/ld+json">{ broken</script>
      <script type="application/ld+json">{"@graph": [{"@type": "TVEpisode", "name": "The Ring",
        "episodeNumber": "3", "partOfSeason": {"seasonNumber": 1},
        "partOfSeries": {"name": "Lanterns"}}]}</script>`;
    expect(structuredData(document)).toEqual({
      episodeName: "The Ring",
      episode: 3,
      season: 1,
      series: "Lanterns",
    });
    document.head.innerHTML = `<script type="application/ld+json">{"@type": "Movie", "name": "Dune",
      "datePublished": "2021-10-22"}</script>`;
    expect(structuredData(document)).toEqual({ series: "Dune", year: "2021" });
  });
});
