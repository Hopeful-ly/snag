/** "1.4 GB", "820 MB", "12 KB"; "" for 0 (unknown). */
export function sizeText(bytes: number): string {
  if (!bytes) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024;
    i++;
  }
  return (i >= 2 ? bytes.toFixed(bytes < 10 ? 1 : 0) : Math.round(bytes)) + " " + units[i];
}

/** A video's length: "45 s", "12 min", "1 h 05 min". */
export function durationText(seconds: number): string {
  if (seconds < 60) return Math.round(seconds) + " s";
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  return h ? `${h} h ${String(m).padStart(2, "0")} min` : `${m} min`;
}

/** Time left: "40s", "3m 20s", "1h 5m". */
export function timeText(seconds: number | null | undefined): string {
  if (seconds == null) return "";
  if (seconds < 60) return seconds + "s";
  if (seconds < 3600) return Math.floor(seconds / 60) + "m " + (seconds % 60) + "s";
  return Math.floor(seconds / 3600) + "h " + Math.floor((seconds % 3600) / 60) + "m";
}

/** An address's site, without "www.": "" if it isn't an address. */
export function hostOf(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return "";
  }
}

/** An http(s) address, as opposed to blob:, data: or a browser page. */
export function isWebAddress(url: string | undefined): url is string {
  return /^https?:/.test(url || "");
}

/** What the card says of a list: "182 in all · Download playlist…". */
export function playlistText(count: number): string {
  return `${count} in all · Download playlist…`;
}
