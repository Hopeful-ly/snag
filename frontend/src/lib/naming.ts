import type { PlaylistEntry, PlaylistView, QueueState } from "./api/client";
import { NameKind } from "../../bindings/github.com/Roham1299/snag/internal/playlist/models";

// Must match config.DefaultNaming.
export const defaultNaming = {
  songs: "{artist} - {title}",
  videos: "{n} - {title}",
  episodes: "{show} S{season}E{episode} - {title}",
};

export function kindOf(view: PlaylistView, quality: string): keyof typeof defaultNaming {
  if (view.kind === NameKind.KindEpisodes) return "episodes";
  if (view.qualities.find((q) => q.id === quality)?.audioOnly) return "songs";
  return "videos";
}

export function templateOf(
  view: PlaylistView,
  queues: QueueState[],
  queue: string,
  quality: string,
): string {
  const named = queues.find((q) => q.id === queue)?.naming;
  if (named) return named;
  const kind = kindOf(view, quality);
  return view.defaults?.[kind] || view.naming || defaultNaming[kind];
}

// Uses dir's kind of slash; an empty name returns dir.
export function joinFolder(dir: string, name: string): string {
  if (!name) return dir;
  const sep = dir.includes("\\") && !dir.includes("/") ? "\\" : "/";
  return dir.endsWith(sep) ? dir + name : dir + sep + name;
}

// Empty fields drop out as in the app: "{artist} - {title}" with no artist is just the title.
export function fillTemplate(t: string, values: Record<string, string>): string {
  const texts: string[] = [];
  const fields: string[] = [];
  let text = "";
  for (let i = 0; i < t.length; i++) {
    const c = t[i]!;
    if ((c === "{" || c === "}") && t[i + 1] === c) {
      text += c;
      i++;
      continue;
    }
    if (c === "{") {
      const end = t.indexOf("}", i);
      if (end < 0) return "";
      texts.push(text);
      fields.push(
        t
          .slice(i + 1, end)
          .trim()
          .toLowerCase(),
      );
      text = "";
      i = end;
      continue;
    }
    if (c === "}") return "";
    text += c;
  }
  if (!fields.length) return "";
  texts.push(text);
  const filled = fields.map((f) => (values[f] ?? "").trim());
  let shown = false;
  for (let i = 0; i < filled.length; i++) {
    if (filled[i]) {
      shown = true;
      continue;
    }
    const open = lastOpen(texts[i]!);
    const after = texts[i + 1] ?? "";
    const shut = closing[open];
    if (shut && after.trimStart().startsWith(shut)) {
      texts[i] = texts[i]!.trimEnd().replace(new RegExp(`\\${open}$`), "");
      texts[i + 1] = after.trimStart().slice(1);
    }
    if (shown) texts[i] = "";
    else if (i + 1 < filled.length) texts[i + 1] = "";
  }
  if (!shown) return "";
  return texts
    .map((s, i) => s + (filled[i] ?? ""))
    .join("")
    .replace(/\s+/g, " ")
    .trim();
}

// {n} is padded as in the app; season, episode and track are two digits.
export function entryPreview(
  template: string,
  entry: PlaylistEntry,
  index: number,
  count: number,
  list: string,
): string {
  const tags = entry.tags;
  const two = (n: number | undefined) => (n && n > 0 ? String(n).padStart(2, "0") : "");
  const date = tags?.date ?? "";
  const credits = fileCredits(
    tags?.artist ?? "",
    tags?.title || entry.title || "",
    tags?.album_artist ?? "",
  );
  return fillTemplate(template, {
    title: credits.title,
    artist: credits.artist,
    album: tags?.album ?? "",
    album_artist: tags?.album_artist ?? "",
    track: two(tags?.track),
    disc: tags?.disc ? String(tags.disc) : "",
    year: date.length >= 4 ? date.slice(0, 4) : "",
    show: tags?.show ?? "",
    season: two(tags?.season),
    episode: two(tags?.episode),
    n: String(index + 1).padStart(Math.max(2, String(count).length), "0"),
    playlist: list,
  });
}

// Must match fileCredits in the app.
const featInTitle =
  /(?:\(|\[)\s*(?:featuring|feat|ft)\.?\s+|(?:^|\s)(?:featuring|feat|ft)\.?\s+\S/i;

// Mirrors the app's fileCredits: the album artist, with the others moved into the title as
// "(feat. A and B)". The full artist list stays when there is no usable album artist.
export function fileCredits(
  artist: string,
  title: string,
  albumArtist: string,
): { artist: string; title: string } {
  const mains = namesOf(albumArtist);
  const all = namesOf(artist);
  if (!mains.length || !all.length || variousArtists(mains)) return { artist, title };
  const main = new Set(mains.map((name) => name.toLowerCase()));
  const featured = all.filter((name) => !main.has(name.toLowerCase()));
  const onAlbum = all.some((name) => main.has(name.toLowerCase()));
  if (!onAlbum || !featured.length) return { artist, title };
  const named = title.trim();
  if (!named || featInTitle.test(named)) return { artist: albumArtist.trim(), title: named };
  return { artist: albumArtist.trim(), title: `${named} (feat. ${joinNames(featured)})` };
}

function namesOf(s: string): string[] {
  return s
    .split(",")
    .map((name) => name.trim())
    .filter(Boolean);
}

function variousArtists(names: string[]): boolean {
  return names.length === 1 && names[0]!.toLowerCase() === "various artists";
}

function joinNames(names: string[]): string {
  if (names.length <= 1) return names[0] ?? "";
  if (names.length === 2) return `${names[0]} and ${names[1]}`;
  return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

const closing: Record<string, string> = { "(": ")", "[": "]", "{": "}" };

function lastOpen(s: string): string {
  s = s.trimEnd();
  return s ? s[s.length - 1]! : "";
}
