import type { Media } from "../shared/protocol";

const MB = 1024 * 1024;

/**
 * Prefers a stream with every quality (HLS master, DASH), else the longest single-quality one
 * (ads are short), else a video file big enough not to be a preview.
 */
export function pickStream(media: Media[]): Media | null {
  const streams = media.filter(
    (m) =>
      (m.media === "hls" || m.media === "dash") &&
      m.info?.type !== "subtitles" &&
      m.info?.type !== "audio" &&
      !m.info?.live,
  );
  const whole = streams.find((m) => m.media === "dash" || m.info?.type === "master");
  if (whole) return whole;
  const longest = [...streams].sort((a, b) => (b.info?.duration ?? 0) - (a.info?.duration ?? 0))[0];
  if (longest) return longest;
  return media.find((m) => m.media === "video" && m.size >= 20 * MB) ?? null;
}

/** A TMDB API mirror's show request: ".../3/tv/95350" or ".../season/1". */
const tmdbShow = /^(.*)\/3\/tv\/\d+(?:\/season\/\d+)?\/?$/;

/** The mirror's base without "/3", keeping any api_key; null when it isn't one. */
export function tmdbCopyOf(address: string): string | null {
  let u: URL;
  try {
    u = new URL(address);
  } catch {
    return null;
  }
  if (u.protocol !== "https:" && u.protocol !== "http:") return null;
  const m = tmdbShow.exec(u.pathname);
  if (!m) return null;
  const key = u.searchParams.get("api_key");
  return u.origin + m[1] + (key ? "?" + new URLSearchParams({ api_key: key }) : "");
}

/** The last mirror among addresses (oldest first), preferring keyed ones; "" for none. */
export function tmdbCopyIn(addresses: string[]): string {
  const copies = addresses.map(tmdbCopyOf).filter((c): c is string => c !== null);
  const keyed = copies.filter((c) => c.includes("?api_key="));
  return keyed[keyed.length - 1] ?? copies[copies.length - 1] ?? "";
}
