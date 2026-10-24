import type { Check, Download, Subscription } from "./api/client";
import { categories, type Category, type CategoryId } from "./filters";

const units = ["B", "KB", "MB", "GB", "TB"];

// Whole KB; one decimal from 1 MB while under 10. "" for 0.
export function size(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "";
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  const text = unit >= 2 ? value.toFixed(value < 10 ? 1 : 0) : String(Math.round(value));
  return `${text} ${units[unit]}`;
}

export function duration(seconds: number | null | undefined): string {
  if (seconds == null) return "";
  const s = Math.round(seconds);
  if (s < 60) return `${s} s`;
  if (s < 3600) return `${Math.floor(s / 60)} min ${s % 60} s`;
  return `${Math.floor(s / 3600)} h ${Math.floor((s % 3600) / 60)} min`;
}

// ms timestamp; anything older than a week shows a date.
export function ago(stamp: number, now = Date.now()): string {
  if (!stamp) return "";
  const s = (now - stamp) / 1000;
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  if (s < 7 * 86400) return `${Math.floor(s / 86400)} d ago`;
  return new Date(stamp).toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

// host("https://www.example.com/a") === "example.com"
export function host(url: string): string {
  return /^https?:\/\/(?:www\.)?([^/:]+)/.exec(url)?.[1] ?? "";
}

export function extension(d: Pick<Download, "name">): string {
  return /\.([a-z0-9]{1,5})$/i.exec(d.name)?.[1]?.toLowerCase() ?? "";
}

const kinds: Record<Exclude<CategoryId, "other">, string[]> = {
  video: [
    "mp4",
    "mkv",
    "webm",
    "mov",
    "avi",
    "flv",
    "m4v",
    "3gp",
    "ts",
    "wmv",
    "ogv",
    "mpg",
    "mpeg",
  ],
  music: ["mp3", "m4a", "ogg", "opus", "flac", "wav", "aac", "wma"],
  archives: ["zip", "rar", "7z", "tar", "gz", "xz", "zst", "bz2", "tgz", "iso"],
  programs: ["exe", "msi", "deb", "rpm", "apk", "appimage", "dmg", "bin", "run", "sh", "flatpak"],
  documents: [
    "pdf",
    "epub",
    "doc",
    "docx",
    "odt",
    "txt",
    "md",
    "xls",
    "xlsx",
    "ppt",
    "pptx",
    "csv",
  ],
};

// Streams count as video unless they are audio.
export function categoryOf(d: Pick<Download, "name" | "label" | "kind">): Category {
  const ext = extension(d);
  let id: CategoryId = "other";
  if (/audio|mp3|m4a/i.test(d.label) || kinds.music.includes(ext)) id = "music";
  else if (kinds.video.includes(ext) || d.kind === "stream") id = "video";
  else if (kinds.archives.includes(ext)) id = "archives";
  else if (kinds.programs.includes(ext)) id = "programs";
  else if (kinds.documents.includes(ext)) id = "documents";
  return categories.find((c) => c.id === id) ?? (categories.at(-1) as Category);
}

// -1 when the size is unknown.
export function percent(d: Pick<Download, "size" | "done">): number {
  return d.size > 0 ? Math.min(100, Math.floor((100 * d.done) / d.size)) : -1;
}

// waiting: why a queued download waits.
export function meta(d: Download, now = Date.now(), waiting = "Waiting for a free slot"): string {
  const total = d.size ? (d.estimated ? "~" : "") + size(d.size) : "";
  const parts: string[] = [];
  switch (d.status) {
    case "downloading":
      if (d.stage) {
        // yt-dlp has it all and is finishing (fixing the container, joining picture and sound).
        const far = stagePercent(d);
        parts.push(`${d.stage}…${far >= 0 ? ` ${far}%` : ""}`, d.size ? total : size(d.done));
        break;
      }
      if (d.note) {
        parts.push(d.note);
        break;
      }
      parts.push(d.size ? `${size(d.done) || "0 B"} of ${total}` : size(d.done) || "Starting…");
      if (d.speed) parts.push(`${size(d.speed)}/s`);
      if (d.eta && d.eta > 0) parts.push(`${duration(d.eta)} left`);
      break;
    case "done":
      if (d.note) {
        parts.push(d.note);
        break;
      }
      if (d.check && d.check.result !== "looking") {
        parts.push(size(d.size), checkText(d.check, d.size));
        break;
      }
      parts.push(size(d.size), d.label, host(d.page || d.url), ago(d.finished, now));
      break;
    case "error":
      parts.push(d.where ? "Try again on YouTube Music, or somewhere else" : d.error || "Failed");
      break;
    case "pending":
      parts.push("Choose where to save it", d.label, total);
      break;
    case "pausing":
      parts.push("Pausing…");
      break;
    case "removing":
      parts.push("Removing…");
      break;
    case "queued":
      parts.push(waiting);
      if (d.size) parts.push(`${size(d.done) || "0 B"} of ${total}`);
      break;
    default:
      parts.push("Paused");
      if (d.size) parts.push(`${size(d.done) || "0 B"} of ${total}`);
      else if (d.done) parts.push(size(d.done));
  }
  return parts.filter(Boolean).join("  ·  ");
}

const foundWhere: Record<string, string> = {
  page: "on the page",
  server: "from its server",
  github: "on GitHub",
  file: "in a checksum file",
  you: "",
};
const cameFrom: Record<string, string> = {
  page: "from the page",
  server: "from its server",
  github: "from GitHub",
  file: "from a checksum file",
  you: "the one you pasted",
};

export function checkText(c: Check, fileSize = 0): string {
  switch (c.result) {
    case "looking":
      return "Looking for a checksum…";
    case "checking": {
      const pct = fileSize > 0 ? ` ${Math.min(100, Math.floor((100 * c.read) / fileSize))}%` : "";
      return `Checking ${c.algo}…${pct}`;
    }
    case "match":
      return `${c.algo} matches (${cameFrom[c.from] ?? "found"})`;
    case "mismatch":
      return `Doesn't match its ${c.algo}: the file may be damaged or changed`;
    case "failed":
      return `Couldn't check it: ${c.error || "the file couldn't be read"}`;
  }
  return `${c.algo} found ${foundWhere[c.from] ?? ""}`.trim();
}

export function checkAbout(c: Check): string {
  if (c.result === "match")
    return c.weak
      ? `The file is what the ${c.algo} checksum describes: it isn't damaged. ${c.algo} can't show that nobody changed it on purpose.`
      : "The file is exactly what the checksum describes: it isn't damaged. A checksum from the same place as the file can't show the place itself is honest.";
  if (c.result === "mismatch") return `Expected ${c.want}\nThe file's ${c.got}`;
  return `${c.algo} ${c.want}`;
}

// A hash of a known length, alone or with its algorithm or file name.
export function looksLikeChecksum(text: string): boolean {
  return /^(?:(?:md5|sha-?(?:1|256|384|512))(?:sum)?\s*[:=]?\s*)?(?:[0-9a-f]{32}|[0-9a-f]{40}|[0-9a-f]{64}|[0-9a-f]{96}|[0-9a-f]{128})(?:\s+\*?\S.*)?$/i.test(
    text.trim(),
  );
}

export function subscriptionLine(s: Subscription, now = Date.now()): string {
  if (s.checking) return "Looking for new ones…";
  if (s.paused) return "Paused";
  if (s.error) return "Can't read it";
  if (!s.checked) return "Not looked at yet";
  const when = ago(s.checked, now);
  return s.last > 0 ? `${s.last} new, ${when}` : `Looked ${when}`;
}

// ms timestamp; "soon" once due.
export function until(stamp: number, now = Date.now()): string {
  const s = (stamp - now) / 1000;
  if (s < 60) return "soon";
  if (s < 3600) return `in ${Math.round(s / 60)} min`;
  if (s < 86400) return `in ${Math.round(s / 3600)} h`;
  return `in ${Math.round(s / 86400)} d`;
}

export function subscriptionKind(s: Pick<Subscription, "source" | "podcast">): string {
  if (s.podcast) return "Podcast";
  const site = host(s.source);
  if (/(^|\.)youtube\.com$|^youtu\.be$/.test(site))
    return /[?&]list=|\/playlist\b/.test(s.source) ? "YouTube playlist" : "YouTube channel";
  if (/(^|\.)spotify\.com$/.test(site)) return "Spotify playlist";
  if (/^music\.apple\.com$/.test(site)) return "Apple Music playlist";
  if (/\.(rss|xml|atom)$|\/feed\b/i.test(s.source)) return "Feed";
  return site || "Playlist";
}

export function subscriptionSource(s: Pick<Subscription, "source" | "podcast">): string {
  const kind = subscriptionKind(s);
  const site = host(s.source);
  return kind === site || /^(YouTube|Spotify|Apple Music) /.test(kind)
    ? kind
    : `${kind}  ·  ${site}`;
}

export function subscriptionStatus(s: Subscription, now = Date.now()): string {
  if (s.checking) return "Looking for new ones now…";
  const parts: string[] = [];
  parts.push(s.checked ? `Looked ${ago(s.checked, now)}` : "Not looked at yet");
  if (s.paused) parts.push("paused");
  else if (s.next) parts.push(`next ${until(s.next, now)}`);
  if (s.last) parts.push(`${s.last} new last time`);
  if (s.added) parts.push(`${s.added} downloaded`);
  return parts.join(", ");
}

export function everyText(hours: number): string {
  if (hours === 1) return "Every hour";
  if (hours === 24) return "Every day";
  if (hours === 168) return "Every week";
  if (hours % 24 === 0) return `Every ${hours / 24} days`;
  return `Every ${hours} hours`;
}

// In milliseconds; 0 unless downloading.
export function quietFor(d: Pick<Download, "status" | "heard">, now = Date.now()): number {
  return d.status === "downloading" && d.heard ? Math.max(0, now - d.heard) : 0;
}

const quietTooLong = 90_000;

type Tone = "busy" | "good" | "warn" | "bad" | "plain";

// quiet is quietFor's result.
export function detailsStatus(d: Download, quiet = 0): { title: string; why: string; tone: Tone } {
  switch (d.status) {
    case "downloading":
      if (d.stage)
        return {
          title: "Finishing",
          why:
            `${d.stage}. ` +
            "Everything has arrived; this step works through the whole file, so a big video " +
            (d.stageOf
              ? "takes a little while on a slow disk."
              : "takes a little while, and the percentage stays where it is meanwhile."),
          tone: "busy",
        };
      if (quiet >= quietTooLong)
        return {
          title: "Waiting for the site",
          why:
            `Nothing has come for ${duration(quiet / 1000)}. The site may have stopped sending; ` +
            "pausing it and resuming usually gets it going again, from where it stopped.",
          tone: "warn",
        };
      return {
        title: "Downloading",
        why:
          d.estimated && percent(d) >= 98
            ? "Its size is a guess until it's done, so it can sit near the end a little while."
            : "",
        tone: "busy",
      };
    case "queued":
      return { title: "Waiting", why: "It starts when its queue has room.", tone: "plain" };
    case "pausing":
      return { title: "Pausing…", why: "", tone: "plain" };
    case "paused":
      return {
        title: "Paused",
        why: "Resume it to carry on from where it stopped.",
        tone: "plain",
      };
    case "pending":
      return { title: "Choose where to save it", why: "", tone: "warn" };
    case "error":
      return { title: "Failed", why: d.error, tone: "bad" };
    case "done":
      return { title: "Done", why: d.note, tone: d.note ? "warn" : "good" };
    case "removing":
      return { title: "Removing…", why: "", tone: "plain" };
  }
  return { title: d.status, why: "", tone: "plain" };
}

// -1 when unknown.
export function stagePercent(d: Pick<Download, "stage" | "stageDone" | "stageOf">): number {
  if (!d.stage || !d.stageOf) return -1;
  return Math.min(99, Math.floor((100 * d.stageDone) / d.stageOf));
}
