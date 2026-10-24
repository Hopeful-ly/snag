// Language of a caption file the page loaded, from its name or address.

const NAMES: Record<string, string> = {
  af: "Afrikaans",
  ar: "Arabic",
  bg: "Bulgarian",
  bn: "Bangla",
  bs: "Bosnian",
  ca: "Catalan",
  cs: "Czech",
  da: "Danish",
  de: "German",
  el: "Greek",
  en: "English",
  es: "Spanish",
  et: "Estonian",
  fa: "Persian",
  fi: "Finnish",
  fr: "French",
  he: "Hebrew",
  hi: "Hindi",
  hr: "Croatian",
  hu: "Hungarian",
  id: "Indonesian",
  is: "Icelandic",
  it: "Italian",
  ja: "Japanese",
  ko: "Korean",
  lt: "Lithuanian",
  lv: "Latvian",
  mk: "Macedonian",
  ms: "Malay",
  my: "Burmese",
  nl: "Dutch",
  no: "Norwegian",
  pl: "Polish",
  pt: "Portuguese",
  ro: "Romanian",
  ru: "Russian",
  sk: "Slovak",
  sl: "Slovenian",
  sq: "Albanian",
  sr: "Serbian",
  sv: "Swedish",
  th: "Thai",
  tr: "Turkish",
  uk: "Ukrainian",
  ur: "Urdu",
  vi: "Vietnamese",
  zh: "Chinese",
};

const ISO3: Record<string, string> = {
  ara: "ar",
  ben: "bn",
  bul: "bg",
  cat: "ca",
  ces: "cs",
  cze: "cs",
  dan: "da",
  deu: "de",
  dut: "nl",
  ell: "el",
  eng: "en",
  fao: "fa",
  fas: "fa",
  fin: "fi",
  fra: "fr",
  fre: "fr",
  ger: "de",
  gre: "el",
  heb: "he",
  hin: "hi",
  hun: "hu",
  ind: "id",
  ita: "it",
  jpn: "ja",
  kor: "ko",
  nld: "nl",
  nor: "no",
  per: "fa",
  pol: "pl",
  por: "pt",
  ron: "ro",
  rum: "ro",
  rus: "ru",
  spa: "es",
  swe: "sv",
  tha: "th",
  tur: "tr",
  ukr: "uk",
  vie: "vi",
  alb: "sq",
  sqi: "sq",
  bos: "bs",
  bur: "my",
  mya: "my",
  est: "et",
  hrv: "hr",
  ice: "is",
  isl: "is",
  lit: "lt",
  lav: "lv",
  mac: "mk",
  mkd: "mk",
  may: "ms",
  msa: "ms",
  slo: "sk",
  slk: "sk",
  slv: "sl",
  srp: "sr",
  urd: "ur",
  chi: "zh",
  zho: "zh",
};

const FULL: Record<string, string> = {
  ...Object.fromEntries(Object.entries(NAMES).map(([code, name]) => [name.toLowerCase(), code])),
  bengali: "bn",
  farsi: "fa",
};

export function looksFilename(name: string): boolean {
  return (
    /\.(srt|vtt|ass|ssa|sub)(\b|$)/i.test(name) ||
    /webrip|bluray|yts\.|1080p|720p|x264|x265/i.test(name) ||
    name.length > 32
  );
}

export function primaryLang(code: string): string {
  const p = (code.split("-")[0] || code).toLowerCase();
  return p || "und";
}

export function captionLabel(code: string, rawName = ""): string {
  const p = primaryLang(code);
  if (p !== "und" && NAMES[p]) return NAMES[p];
  const fromRaw = langIn(rawName);
  if (fromRaw && NAMES[fromRaw]) return NAMES[fromRaw];
  if (rawName && !looksFilename(rawName) && rawName.length < 28) return rawName;
  return "Captions";
}

/** Captions the player already has on this video: a <track src>, or cues in memory. */
export function tracksOf(video: HTMLVideoElement | null): {
  code: string;
  name: string;
  url?: string;
  text?: string;
  showing?: boolean;
}[] {
  if (!video) return [];
  const out: {
    code: string;
    name: string;
    url?: string;
    text?: string;
    showing?: boolean;
  }[] = [];
  const seen = new Set<string>();
  const add = (
    code: string,
    rawName: string,
    extra: { url?: string; text?: string; showing?: boolean },
  ) => {
    const resolved = langOf(code) || langIn(rawName) || captionLang(extra.url || "", rawName).code;
    const k = extra.url || extra.text || resolved;
    if (!k || seen.has(k)) return;
    seen.add(k);
    out.push({
      code: resolved || "und",
      name: captionLabel(resolved, rawName),
      ...extra,
    });
  };
  const nodes = [...video.querySelectorAll("track")];
  const catalog = catalogTracks(nodes);
  for (const el of nodes) {
    const src = el.src;
    if (!src || !/^https?:/i.test(src)) continue;
    const loaded = trackLoaded(el.track);
    // A search catalog (OpenSubtitles and the like) is not this video's captions.
    if (catalog && !loaded) continue;
    add(el.srclang, el.label || "", {
      url: src,
      showing: el.track?.mode === "showing",
    });
  }
  try {
    for (const track of video.textTracks) {
      if (track.kind !== "subtitles" && track.kind !== "captions") {
        continue;
      }
      const cues = track.cues;
      if (!cues || cues.length === 0) continue;
      const rows: { startTime: number; endTime: number; text: string }[] = [];
      for (let i = 0; i < cues.length; i++) {
        const c = cues[i]!;
        const text = "text" in c ? String((c as VTTCue).text) : "";
        if (text) rows.push({ startTime: c.startTime, endTime: c.endTime, text });
      }
      const srt = cuesToSRT(rows);
      if (!srt) continue;
      add(track.language, track.label || "", {
        text: srt,
        showing: track.mode === "showing",
      });
    }
  } catch {
    // some players lock textTracks
  }
  return out;
}

/** Many unused <track> tags from a subtitle search, not the two languages this video ships. */
export function catalogTracks(nodes: Pick<HTMLTrackElement, "src" | "label">[]): boolean {
  if (nodes.length <= 2) return false;
  let hits = 0;
  for (const el of nodes) {
    const label = el.label || "";
    if (/opensubtitles/i.test(el.src) || looksFilename(label) || /\([^)]+\)/.test(label)) hits++;
  }
  return hits >= 3;
}

function trackLoaded(track: TextTrack | null | undefined): boolean {
  if (!track) return false;
  if (track.mode === "showing") return true;
  return !!track.cues && track.cues.length > 0;
}

export function cuesToSRT(cues: { startTime: number; endTime: number; text: string }[]): string {
  if (!cues.length) return "";
  return cues
    .map(
      (c, i) => `${i + 1}\n${srtTime(c.startTime)} --> ${srtTime(c.endTime)}\n${c.text.trim()}\n`,
    )
    .join("\n");
}

function srtTime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) sec = 0;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  const ms = Math.round((sec % 1) * 1000);
  const pad = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${pad(h)}:${pad(m)}:${pad(s)},${pad(ms, 3)}`;
}

export function captionLang(url: string, filename?: string): { code: string; name: string } {
  const fromName = langIn(filename || "");
  if (fromName) return named(fromName);
  try {
    const u = new URL(url);
    // tlang is the language a caption is translated into, which is the one it is in.
    for (const key of ["tlang", "lang", "language", "l", "sub_lang", "sublanguageid", "label"]) {
      const got = langOf(u.searchParams.get(key) || "");
      if (got) return named(got);
    }
    const fromPath = langIn(u.pathname);
    if (fromPath) return named(fromPath);
  } catch {
    // not an address
  }
  return { code: "und", name: "Captions" };
}

function named(code: string): { code: string; name: string } {
  return { code, name: captionLabel(code) };
}

function langOf(raw: string): string {
  const s = raw.trim().toLowerCase();
  if (!s) return "";
  const full = FULL[s];
  if (full) return full;
  const base = s.split(/[-_]/)[0] || "";
  if (NAMES[base]) return base;
  return ISO3[base] || "";
}

// The last known language token in a name: Movie.en.srt, /subtitles/ar.vtt, english.srt.
function langIn(text: string): string {
  let found = "";
  const tokens = text.toLowerCase().split(/[^a-z0-9]+/);
  for (const tok of tokens) {
    const got = langOf(tok);
    if (got) found = got;
  }
  return found;
}
