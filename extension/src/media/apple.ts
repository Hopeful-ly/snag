// Apple Music's public pages. Its sound is protected, so a list opens in Snag and each song is
// found elsewhere, labelled as Apple Music names it.

/** What an Apple Music page is of. An album's ?i= track, and a /song/ page, are one song. */
export type AppleList = "playlist" | "album" | "song" | "artist";

const listPage = /^\/([a-z]{2})\/(album|song|playlist|artist)\/[^/]+\/([^/]+)\/?$/;
const playlistID = /^pl\.[0-9a-f]+$/i;
const numericID = /^[0-9]{2,}$/;

export function appleListOf(address: string | undefined): AppleList | null {
  if (!address) return null;
  try {
    const u = new URL(address);
    if (!isAppleHost(u.hostname)) return null;
    const m = listPage.exec(u.pathname);
    if (!m) return null;
    const kind = m[2] as AppleList | undefined;
    const id = m[3];
    if (!kind || !id) return null;
    if (kind === "playlist") return playlistID.test(id) ? "playlist" : null;
    if (!numericID.test(id)) return null;
    if (kind === "album" && numericID.test(u.searchParams.get("i") || "")) return "song";
    return kind === "song" ? "song" : kind;
  } catch {
    return null;
  }
}

export function isAppleList(address: string | undefined): boolean {
  return appleListOf(address) != null;
}

export function isAppleMusic(address: string | undefined): boolean {
  try {
    return !!address && isAppleHost(new URL(address).hostname);
  } catch {
    return false;
  }
}

function isAppleHost(host: string): boolean {
  return (
    host === "music.apple.com" || host === "geo.music.apple.com" || host === "embed.music.apple.com"
  );
}
