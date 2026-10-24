import { beforeAll, describe, expect, it, vi } from "vitest";
import { headersOf, isSpotify, isSpotifyList, queryOf, spotifyListOf } from "../src/media/spotify";

type Listener = (details: Record<string, unknown>) => void;

// The browser's webRequest events, keeping the listeners added.
const listeners: Record<string, Listener> = {};
const event = (name: string) => ({
  addListener: vi.fn((l: Listener) => (listeners[name] = l)),
});

let background: typeof import("../src/background/spotify");
beforeAll(async () => {
  vi.stubGlobal("chrome", {
    webRequest: {
      onBeforeRequest: event("request"),
      onBeforeSendHeaders: event("headers"),
    },
    storage: {},
  });
  background = await import("../src/background/spotify");
  background.watchSpotify();
});

const body = (q: unknown) => new TextEncoder().encode(JSON.stringify(q)).buffer;
const fetchPlaylist = {
  variables: { uri: "spotify:playlist:37i9dQZF1DX4o1oenSJRJd", offset: 0, limit: 25 },
  operationName: "fetchPlaylist",
  extensions: { persistedQuery: { version: 1, sha256Hash: "21ea11a6" } },
};

describe("Spotify's pages", () => {
  it("are told apart from the rest", () => {
    expect(isSpotifyList("https://open.spotify.com/playlist/37i9dQZF1DX4o1oenSJRJd")).toBe(true);
    expect(isSpotifyList("https://open.spotify.com/intl-de/album/4m2880jivSbbyEGAKfITCa")).toBe(
      true,
    );
    expect(isSpotifyList("https://open.spotify.com/track/3zOYmiNOuyDL0ClxJcEJ0T?si=x")).toBe(true);
    expect(isSpotifyList("https://open.spotify.com/")).toBe(false);
    expect(isSpotifyList("https://example.com/playlist/37i9dQZF1DX4o1oenSJRJd")).toBe(false);
    expect(spotifyListOf("https://open.spotify.com/collection/tracks")).toBe("liked");
    expect(spotifyListOf("https://open.spotify.com/artist/4tZwfgrHOc3mvqYlEYSvVi")).toBe("artist");
    expect(spotifyListOf("https://open.spotify.com/album/4m2880jivSbbyEGAKfITCa")).toBe("album");
    expect(spotifyListOf("https://open.spotify.com/collection/albums")).toBeNull();
    expect(spotifyListOf("https://open.spotify.com/search/daft")).toBeNull();
    expect(isSpotify("https://open.spotify.com/")).toBe(true);
    expect(isSpotify("not a link")).toBe(false);
  });

  it("read the player's queries and how it sends them", () => {
    expect(queryOf(body(fetchPlaylist))).toEqual({ name: "fetchPlaylist", hash: "21ea11a6" });
    expect(queryOf(body({ operationName: "x" }))).toBeNull();
    expect(queryOf(new TextEncoder().encode("not json").buffer)).toBeNull();
    expect(
      headersOf([
        { name: "authorization", value: "Bearer abc" },
        { name: "client-token", value: "ct" },
        { name: "Spotify-App-Version", value: "1.2.3" },
        { name: "User-Agent", value: "UA" },
      ]),
    ).toEqual({ token: "Bearer abc", client_token: "ct", app_version: "1.2.3", user_agent: "UA" });
    expect(headersOf([{ name: "content-type", value: "application/json" }])).toBeNull();
  });
});

describe("a tab's Spotify session", () => {
  it("is kept once the page has asked with a token and a query, and forgotten with it", async () => {
    expect(await background.spotifyOf(4)).toBeUndefined();
    listeners.request!({ tabId: 4, requestBody: { raw: [{ bytes: body(fetchPlaylist) }] } });
    expect(await background.spotifyOf(4)).toBeUndefined(); // no token yet
    listeners.headers!({
      tabId: 4,
      requestHeaders: [
        { name: "authorization", value: "Bearer abc" },
        { name: "client-token", value: "ct" },
      ],
    });
    expect(await background.spotifyOf(4)).toMatchObject({
      token: "Bearer abc",
      client_token: "ct",
      hashes: { fetchPlaylist: "21ea11a6" },
    });
    expect(await background.spotifyOf(5)).toBeUndefined(); // another tab's
    background.forgetSpotify(4);
    expect(await background.spotifyOf(4)).toBeUndefined();
  });
});
