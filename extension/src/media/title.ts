// Names what a page plays: "Series S01E03 - Episode name", "Movie (2024)", else the page title
// without the site name and padding. collect() reads the page; compose() is pure logic.

export interface Clues {
  url: string;
  host: string;
  docTitle?: string;
  ogTitle?: string;
  h1?: string;
  session?: { title?: string; artist?: string; album?: string };
  ld?: StructuredData;
  headings?: string[];
}

export interface StructuredData {
  series?: string;
  season?: number;
  episode?: number;
  episodeName?: string;
  year?: string;
}

interface Named {
  title: string;
  series?: string;
  season?: number;
  episode?: number;
  episodeName?: string;
}

interface Numbers {
  season: number;
  episode: number;
}

const pad = (n: number) => String(n).padStart(2, "0");
const NOISE =
  /^(watch|stream(ing)?|online|free|hd|4k|full (movie|episode)s?|movies?|tv( shows?)?|series|home)$/i;

/** Season and episode in a piece of text: S01E03, S1 E3, 1x03, Season 1 Episode 3, ... */
export function numbersIn(text: string | undefined): Numbers | null {
  if (!text) return null;
  let m = /\bS(?:eason)?\s*0*(\d{1,2})\s*[.\-_ ]?\s*E(?:p(?:isode)?)?\s*0*(\d{1,4})\b/i.exec(text);
  if (m) return { season: +m[1]!, episode: +m[2]! };
  m = /\bseason\s*0*(\d{1,2})\b[^0-9]{0,24}?\b(?:episode|ep\.?|e)\s*0*(\d{1,4})\b/i.exec(text);
  if (m) return { season: +m[1]!, episode: +m[2]! };
  m = /(?:^|[^0-9a-z])0*(\d{1,2})x0*(\d{1,3})(?![0-9])/i.exec(text);
  if (m && +m[1]! > 0 && +m[1]! < 60) return { season: +m[1]!, episode: +m[2]! };
  return null;
}

/** Season and episode in the address: /tv/95350/1/3, /season/1/episode/3, ?season=1&episode=3. */
export function numbersInUrl(url: string): Numbers | null {
  let u: URL;
  try {
    u = new URL(url);
  } catch {
    return null;
  }
  const path = decodeURIComponent(u.pathname);
  let m =
    /\/(?:tv|series|serie|shows?|watch|episodes?|tv-shows?)\/(?:[^/]+\/){0,2}?(\d{1,2})\/(\d{1,4})\/?$/i.exec(
      path,
    );
  if (m) return { season: +m[1]!, episode: +m[2]! };
  m = /season[-_/ ]?0*(\d{1,2})\D{0,12}?episode[-_/ ]?0*(\d{1,4})/i.exec(path);
  if (m) return { season: +m[1]!, episode: +m[2]! };
  const fromText = numbersIn(path.replace(/[-_/]/g, " "));
  if (fromText) return fromText;
  const q = u.searchParams;
  const s = q.get("season") || q.get("s") || q.get("ss");
  const e = q.get("episode") || q.get("e") || q.get("ep");
  if (s && e && /^\d{1,2}$/.test(s) && /^\d{1,4}$/.test(e)) return { season: +s, episode: +e };
  return null;
}

function siteName(host: string): string {
  return (host || "")
    .replace(/^www\./, "")
    .split(".")[0]!
    .toLowerCase();
}

/** The parts of a page title, without the site's name and padding words. */
function titleParts(text: string | undefined, host: string): string[] {
  if (!text) return [];
  const site = siteName(host);
  return text
    .split(/\s+[|\-–—•·]\s+|\s+::\s+|\s*\|\s*/)
    .map((p) =>
      p
        .trim()
        .replace(/^(watch|stream)\s+/i, "")
        .replace(
          /(\s+(online|for free|free|in hd|hd|full (movie|episode)|with subtitles|subbed|dubbed))+$/i,
          "",
        )
        .trim(),
    )
    .filter((p) => p && !NOISE.test(p) && p.toLowerCase().replace(/[\s.]/g, "") !== site);
}

/** A name without its season/episode text: "Lanterns Season 1 Episode 3" -> "Lanterns". */
function withoutNumbers(text: string): string {
  return (text || "")
    .replace(/\bS(?:eason)?\s*\d{1,2}\s*[.\-_ ]?\s*E(?:p(?:isode)?)?\s*\d{1,4}\b.*$/i, "")
    .replace(/\bseason\s*\d{1,2}\b.*$/i, "")
    .replace(/\bepisode\s*\d{1,4}\b.*$/i, "")
    .replace(/(?:^|\s)\d{1,2}x\d{1,3}\b.*$/i, "")
    .replace(/[\s:.\-–—|,(]+$/, "")
    .trim();
}

/** "Episode 3: The Pilot", "E03 - The Pilot", "3. The Pilot" -> "The Pilot" (for that episode). */
function episodeNameIn(text: string, episode: number): string {
  if (!text) return "";
  const m = new RegExp(
    "^\\s*(?:S\\d{1,2}\\s*)?(?:e|ep\\.?|episode)?\\s*0*" +
      episode +
      "\\s*[:.\\-–—)]\\s*(.{2,90}?)\\s*$",
    "i",
  ).exec(text);
  return m ? m[1]!.trim() : "";
}

function generic(name: string | undefined, series: string): boolean {
  return (
    !name ||
    /^(episode|ep\.?|chapter|part)\s*\d+$/i.test(name.trim()) ||
    name.trim().toLowerCase() === (series || "").trim().toLowerCase()
  );
}

export function compose(s: Clues): Named {
  const ld = s.ld || {};
  const session = s.session || {};
  const titles = [s.ogTitle, s.docTitle, s.h1, session.title].filter((t): t is string => !!t);

  let numbers: Numbers | null =
    ld.season && ld.episode ? { season: ld.season, episode: ld.episode } : null;
  for (const t of titles) if (!numbers) numbers = numbersIn(t);
  if (!numbers) numbers = numbersInUrl(s.url);

  let series = ld.series || "";
  if (!series) {
    for (const t of [s.ogTitle, s.docTitle, s.h1, session.album]) {
      const first = titleParts(t, s.host)
        .map(withoutNumbers)
        .find((p) => p.length > 1);
      if (first) {
        series = first;
        break;
      }
    }
  }
  if (!series) series = withoutNumbers(session.title || "") || (s.docTitle || "").trim();

  let episodeName = "";
  if (numbers) {
    if (!generic(ld.episodeName, series)) episodeName = ld.episodeName!;
    // In a page title after the numbers: "Lanterns - S01E03 - The Pilot | Site".
    for (const t of titles) {
      if (episodeName) break;
      const parts = titleParts(t, s.host);
      const at = parts.findIndex((p) => numbersIn(p));
      if (at >= 0) {
        const after = parts[at]!.replace(
          /^.*?\b(?:S\d{1,2}\s*E\d{1,4}|\d{1,2}x\d{1,3}|episode\s*\d{1,4})\b\s*[:.\-–—]?\s*/i,
          "",
        ).trim();
        const candidate = after || parts[at + 1] || "";
        if (!generic(candidate, series) && !numbersIn(candidate)) episodeName = candidate;
      }
    }
    // The player's media session often names the episode.
    if (!episodeName && session.title) {
      const rest = withoutNumbers(session.title.replace(series, "").replace(/^[\s:.\-–—|]+/, ""));
      const named = episodeNameIn(session.title, numbers.episode) || rest;
      if (!generic(named, series) && named.length > 1 && !numbersIn(named)) episodeName = named;
    }
    // A heading or list item like "E3 - The Pilot" for this episode.
    for (const h of s.headings || []) {
      if (episodeName) break;
      const named = episodeNameIn(h, numbers.episode);
      if (named && !generic(named, series)) episodeName = named;
    }
  }

  const clean = (t: string) =>
    t
      .replace(/[\\/:*?"<>|]+/g, " ")
      .replace(/\s+/g, " ")
      .trim();
  let title = clean(series);
  if (numbers) {
    title += " S" + pad(numbers.season) + "E" + pad(numbers.episode);
    if (episodeName) title += " - " + clean(episodeName);
  } else if (ld.year && !/\(\d{4}\)$/.test(title)) {
    title += " (" + ld.year + ")";
  }
  return {
    title,
    series,
    season: numbers?.season || 0,
    episode: numbers?.episode || 0,
    episodeName,
  };
}

type JsonNode = Record<string, unknown>;

export function structuredData(doc: Document): StructuredData {
  const out: StructuredData = {};
  const obj = (v: unknown): JsonNode => (v && typeof v === "object" ? (v as JsonNode) : {});
  const str = (v: unknown) => (typeof v === "string" ? v : "");
  const visit = (node: unknown): void => {
    if (!node || typeof node !== "object") return;
    if (Array.isArray(node)) return node.forEach(visit);
    const n = node as JsonNode;
    const types = ([] as unknown[]).concat(n["@type"] || []).map(String);
    if (types.some((t) => /Episode$/i.test(t))) {
      out.episodeName = out.episodeName || str(n.name);
      out.episode = out.episode || Number(n.episodeNumber) || 0;
      const season = obj(n.partOfSeason);
      out.season = out.season || Number(season.seasonNumber) || 0;
      const series = obj(n.partOfSeries ?? n.partOfTVSeries ?? season.partOfSeries);
      out.series = out.series || str(series.name);
    } else if (types.some((t) => /^(TVSeries|TVSeason|CreativeWorkSeries)$/i.test(t))) {
      out.series = out.series || str(n.name);
    } else if (types.some((t) => /^Movie$/i.test(t))) {
      out.series = out.series || str(n.name);
      const year = /\b(19|20)\d{2}\b/.exec(String(n.datePublished || n.dateCreated || ""));
      if (year) out.year = year[0];
    }
    if (n["@graph"]) visit(n["@graph"]);
  };
  for (const script of doc.querySelectorAll('script[type="application/ld+json"]')) {
    try {
      visit(JSON.parse(script.textContent || ""));
    } catch {
      // a page's broken data is no reason to fail
    }
  }
  return out;
}

const HEADINGS =
  "h1, h2, h3, h4, [class*='episode' i] [class*='title' i], [class*='episode' i].active, [aria-current], .active";

function collect(doc: Document = document, where: Location = location): Clues {
  const meta = (name: string) =>
    doc.querySelector<HTMLMetaElement>(`meta[property="${name}"], meta[name="${name}"]`)?.content ||
    "";
  let session: Clues["session"] = {};
  try {
    const m = navigator.mediaSession?.metadata;
    if (m) session = { title: m.title || "", artist: m.artist || "", album: m.album || "" };
  } catch {
    // some pages replace mediaSession with something odd
  }
  const headings: string[] = [];
  for (const el of doc.querySelectorAll(HEADINGS)) {
    const text = (el.textContent || "").replace(/\s+/g, " ").trim();
    if (text && text.length < 120) headings.push(text);
    if (headings.length > 200) break;
  }
  return {
    url: where.href,
    host: where.hostname,
    docTitle: doc.title || "",
    ogTitle: meta("og:title") || meta("twitter:title"),
    h1: (doc.querySelector("h1")?.textContent || "").replace(/\s+/g, " ").trim(),
    session,
    ld: structuredData(doc),
    headings,
  };
}

export function read(): Named {
  try {
    return compose(collect());
  } catch {
    return { title: (document.title || "").trim() };
  }
}
