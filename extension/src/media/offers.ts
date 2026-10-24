// The card's list: the site's qualities, the qualities of streams the page loaded, and files.
import { captionLabel, captionLang, looksFilename, primaryLang } from "./captions";
import { durationText, hostOf, sizeText } from "../shared/format";
import type { Item, Media, ProbeResult, Subtitle } from "../shared/protocol";

export interface Caption extends Subtitle {
  url?: string;
  text?: string;
  showing?: boolean;
  /** The site gives these to its own player alone, and it hasn't loaded them. */
  needsPlayer?: boolean;
}

interface Row {
  label: string;
  detail: string;
  item: Item;
}

/** title is null for a lone file or stream. */
interface Group {
  title: string | null;
  rows: Row[];
}

export interface Option extends Row {
  key: string;
  height: number;
  format: string;
  size: number;
  name?: string;
  best?: boolean;
}

interface Offers {
  video: Option[];
  audio: Option[];
  files: Option[];
}

export type StreamProbes = Record<string, { loading: boolean; result: ProbeResult | null }>;

const isStream = (m: { media?: string }) => m.media === "hls" || m.media === "dash";

function detailOf(detail: string, size: number): string {
  return [detail, size ? "~" + sizeText(size) : ""].filter(Boolean).join(" · ");
}

function choiceRows(probe: ProbeResult): Row[] {
  return (probe.choices ?? []).map((c) => ({
    label: c.label,
    detail: detailOf(c.detail, c.size),
    item: {
      url: probe.url || "",
      kind: "ytdlp",
      format: c.format,
      sort: c.sort,
      audio: c.audio,
      size: c.size,
      label: c.label + (c.label === "Audio" ? " " + c.detail : ""),
      title: probe.title,
    },
  }));
}

function bare(url: string): string {
  try {
    const u = new URL(url);
    return u.origin + u.pathname;
  } catch {
    return url;
  }
}

function streamTitle(m: Media): string {
  return [
    "Stream",
    m.media.toUpperCase(),
    m.info?.duration ? durationText(m.info.duration) : "",
    hostOf(m.url),
  ]
    .filter(Boolean)
    .join(" · ");
}

/**
 * Leaves out streams a master playlist already lists, subtitles, and short streams (ads,
 * previews) next to a full-length one.
 */
export function groups(media: Media[], probes: StreamProbes): Group[] {
  // Playlists listed inside ANOTHER playlist. A single-quality playlist lists only itself, which
  // must not hide it.
  const inside = new Set<string>();
  for (const [url, p] of Object.entries(probes)) {
    if (!p.result?.supported) continue;
    for (const v of p.result.variants || []) if (bare(v) !== bare(url)) inside.add(bare(v));
  }
  const longest = Math.max(0, ...media.map((m) => m.info?.duration || 0));
  const out: Group[] = [];
  for (const m of media) {
    const stream = isStream(m);
    if (m.media === "subtitles") continue;
    if (stream && inside.has(bare(m.url))) continue;
    if (stream && m.info?.type === "subtitles") continue;
    if (stream && longest >= 600 && m.info?.duration && m.info.duration < 180) continue;
    const p = probes[m.url];
    const choices = p?.result?.supported ? (p.result.choices ?? []) : [];
    if (stream && choices.length) {
      out.push({
        title: streamTitle(m),
        rows: choices.map((c) => ({
          label: c.label,
          detail: detailOf(c.detail, c.size),
          item: {
            ...m,
            format: c.format,
            audio: c.audio,
            size: c.size,
            label: c.label + (c.label === "Audio" ? " " + c.detail : ""),
          },
        })),
      });
    } else if (stream) {
      const detail = [
        m.media.toUpperCase(),
        m.info?.duration ? durationText(m.info.duration) : "",
        hostOf(m.url),
        p?.loading ? "finding qualities…" : "best quality",
      ];
      out.push({
        title: null,
        rows: [
          {
            label: m.info?.type === "audio" ? "Audio" : "Stream",
            detail: detail.filter(Boolean).join(" · "),
            item: m,
          },
        ],
      });
    } else {
      const ext = (
        new URL(m.url).pathname.match(/\.([a-z0-9]{2,4})$/i)?.[1] ||
        m.mime.split("/")[1] ||
        "file"
      ).toUpperCase();
      const detail = [ext, sizeText(m.size), m.filename || hostOf(m.url)];
      out.push({
        title: null,
        rows: [
          {
            label: m.media === "audio" ? "Audio" : "Video",
            detail: detail.filter(Boolean).join(" · "),
            item: m,
          },
        ],
      });
    }
  }
  return out;
}

export function probeStreams(
  media: Media[],
  probes: StreamProbes,
  probe: (url: string) => Promise<ProbeResult>,
  changed: () => void,
): void {
  for (const m of media) {
    if (!isStream(m) || probes[m.url]) continue;
    probes[m.url] = { loading: true, result: null };
    probe(m.url)
      .then(
        (result) => (probes[m.url] = { loading: false, result }),
        () => (probes[m.url] = { loading: false, result: null }),
      )
      .then(changed);
  }
}

export function qualityName(height: number): string {
  if (height >= 2160) return "4K";
  if (height >= 1440) return "QHD";
  if (height >= 1080) return "Full HD";
  if (height >= 720) return "HD";
  return height > 0 ? "SD" : "";
}

function heightIn(text: string): number {
  const m = /(?:^|[^0-9])(2160|1440|1080|720|576|540|480|360|240|144)p/i.exec(text || "");
  return m ? +m[1]! : 0;
}

export function captionsOf(
  probe: ProbeResult | null,
  streams: StreamProbes,
  media: Media[] = [],
  extra: Caption[] = [],
): Caption[] {
  const all: Caption[] = [...(probe?.subtitles ?? []), ...extra].map(resolveCaption);
  for (const p of Object.values(streams)) {
    if (p.result?.subtitles) all.push(...p.result.subtitles.map(resolveCaption));
  }
  for (const m of media) {
    if (m.media === "subtitles") all.push(resolveCaption(captionOfFile(m)));
  }
  const best = new Map<string, Caption>();
  for (const s of all) {
    const k = primaryLang(s.code);
    // "Captions" with no language is a file we couldn't name, not a choice.
    if (!k || k === "und") continue;
    const prev = best.get(k);
    // Later wins a tie so a newly selected English file replaces an earlier one.
    if (!prev || captionScore(s) >= captionScore(prev)) {
      best.set(k, { ...s, code: k, name: captionLabel(k, s.name) });
    }
  }
  const out = [...best.values()];
  out.sort((a, b) => {
    if (a.code === "en" || b.code === "en") return a.code === "en" ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
  return out;
}

/** YouTube gives captions only to its own player, so only those it loaded come with a file. */
export function markNeedsPlayer(captions: Caption[], page: string): Caption[] {
  let host: string;
  try {
    host = new URL(page).hostname;
  } catch {
    return captions;
  }
  if (!/(^|\.)(youtube\.com|youtube-nocookie\.com|youtu\.be)$/i.test(host)) return captions;
  return captions.map((c) => (c.url || c.text ? c : { ...c, needsPlayer: true }));
}

function captionScore(c: Caption): number {
  return (
    (c.showing ? 8 : 0) +
    (c.text ? 4 : 0) +
    (primaryLang(c.code) !== "und" ? 2 : 0) +
    (c.url ? 1 : 0) -
    (c.auto ? 4 : 0) -
    (looksFilename(c.name) ? 2 : 0)
  );
}

function resolveCaption(s: Caption): Caption {
  const p = primaryLang(s.code);
  if (p !== "und") return { ...s, code: p, name: captionLabel(p, s.name) };
  const { code, name } = captionLang(s.url || "", s.name);
  return { ...s, code, name };
}

function captionOfFile(m: Media): Caption {
  const { code, name } = captionLang(m.url, m.filename);
  return { code, name, url: m.url };
}

function keyOf(r: Row): string {
  const i = r.item;
  return (i.url || "") + "|" + (i.format || "") + "|" + (i.audio || "") + "|" + r.label;
}

/** Video qualities, highest first, one per resolution; then audio; then other files. */
export function options(probe: ProbeResult | null, media: Media[], probes: StreamProbes): Offers {
  const video: Option[] = [];
  const audio: Option[] = [];
  const files: Option[] = [];
  const add = (r: Row, height: number) => {
    const isAudio =
      /^audio$/i.test(r.label) || (r.item.media === "audio" && r.item.kind !== "http");
    const format = (/\b(MP4|MKV|WEBM|M4A|MP3|MOV)\b/.exec(r.detail) || [""])[0] || "MP4";
    (isAudio ? audio : video).push({ ...r, height, format, size: r.item.size || 0, key: keyOf(r) });
  };
  if (probe?.supported) for (const r of choiceRows(probe)) add(r, heightIn(r.label));
  for (const g of groups(media, probes)) {
    for (const r of g.rows) {
      if (!isStream(r.item) && r.item.kind === "http") {
        files.push({ ...r, height: 0, format: "", size: r.item.size || 0, key: r.item.url });
        continue;
      }
      // A stream without its qualities yet: its address may say ("index-s1080p-...").
      const height = heightIn(r.label) || heightIn(new URL(r.item.url).pathname);
      add(height && r.label === "Stream" ? { ...r, label: height + "p" } : r, height);
    }
  }
  video.sort((a, b) => b.height - a.height || b.size - a.size);
  const seen = new Set<number | string>();
  const unique = video.filter((o) => {
    const k = o.height || o.key;
    if (seen.has(k)) return false;
    seen.add(k);
    return true;
  });
  for (const o of unique) o.name = qualityName(o.height);
  if (unique[0]) unique[0].best = true;
  const seenAudio = new Set<string>();
  const audioUnique = audio.filter((o) => {
    const k = o.format + o.item.audio;
    if (seenAudio.has(k)) return false;
    seenAudio.add(k);
    return true;
  });
  return { video: unique, audio: audioUnique, files };
}
