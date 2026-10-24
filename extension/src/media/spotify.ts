// Spotify pages and how the web player calls Spotify's API, so Snag can read whole playlists.
// Songs are found on YouTube Music, as Spotify's audio is protected.
import type { SpotifySession } from "../shared/protocol";

/** What a Spotify list's page is of: an artist's is their top songs, "liked" one's Liked Songs. */
export type SpotifyList = "playlist" | "album" | "track" | "artist" | "liked";

/** A Spotify list's page ("https://open.spotify.com/playlist/…", "/collection/tracks"). */
const listPage =
  /^\/(?:intl-[a-z]{2}(?:-[A-Za-z]{2})?\/)?(?:embed\/)?(?:(playlist|album|track|artist)\/[A-Za-z0-9]{22}|(collection\/tracks))\/?$/;

export function spotifyListOf(address: string | undefined): SpotifyList | null {
  if (!address) return null;
  try {
    const u = new URL(address);
    const m = u.hostname === "open.spotify.com" ? listPage.exec(u.pathname) : null;
    if (!m) return null;
    return m[2] ? "liked" : (m[1] as SpotifyList);
  } catch {
    return null;
  }
}

export function isSpotifyList(address: string | undefined): boolean {
  return spotifyListOf(address) != null;
}

export function isSpotify(address: string | undefined): boolean {
  try {
    return !!address && new URL(address).hostname === "open.spotify.com";
  } catch {
    return false;
  }
}

export const SPOTIFY_API = "https://api-partner.spotify.com/pathfinder/*";

export function queryOf(body: ArrayBuffer | undefined): { name: string; hash: string } | null {
  if (!body) return null;
  try {
    const q = JSON.parse(new TextDecoder().decode(body));
    const name = q?.operationName;
    const hash = q?.extensions?.persistedQuery?.sha256Hash;
    return typeof name === "string" && typeof hash === "string" && name && hash
      ? { name, hash }
      : null;
  } catch {
    return null;
  }
}

export function headersOf(
  headers: { name: string; value?: string }[] | undefined,
): Omit<SpotifySession, "hashes"> | null {
  const get = (name: string) =>
    headers?.find((h) => h.name.toLowerCase() === name)?.value?.trim() || "";
  const token = get("authorization");
  if (!/^Bearer \S+$/.test(token)) return null;
  return {
    token,
    client_token: get("client-token"),
    app_version: get("spotify-app-version"),
    user_agent: get("user-agent") || navigator.userAgent,
  };
}
