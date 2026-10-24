// Tells media apart from everything else a page loads. Pure logic, so it can be tested alone.
import type { Kind, MediaType, PlaylistInfo, SeenEntry, StreamType } from "../shared/protocol";

export interface Response {
  tabId: number;
  statusCode: number;
  url: string;
  type: string;
  responseHeaders?: { name: string; value?: string }[];
}

interface Found {
  media: MediaType;
  kind: Kind;
  mime: string;
  size: number;
  filename?: string;
  url?: string; // the address to fetch it from, when not the one the page used
}

// Adaptive-stream segments. HLS also ships captions as tiny .vtt segments (VTT_SEGMENT).
const FRAGMENT =
  /(\.(ts|m4s|m4f|cmfv|cmfa|aac|key)$)|([/_-](seg|segment|frag|fragment|chunk|init)[-_]?\d*\.(mp4|m4a|webm)$)/i;
const VTT_SEGMENT =
  /[/_-](seg|segment|frag|fragment|chunk|part|files?sequence)[-_]?\d+\.(vtt|webvtt)$/i;
const CAPTION_EXT = /\.(srt|vtt|webvtt|ass|ssa|ttml)$/i;
const CAPTION_MIME =
  /^(text\/vtt|text\/srt|text\/x-ssa|application\/x-subrip|application\/ttml\+xml)(\s|;|$)/i;
const CAPTION_HOST = /(^|\.)(opensubtitles\.(org|com)|opensubtitles\.com)$/i;
const SKIP_HOSTS =
  /(^|\.)(googlevideo\.com|doubleclick\.net|googlesyndication\.com|imasdk\.googleapis\.com)$/i;
const MEDIA_EXT = /\.(mp4|m4v|webm|mkv|mov|flv|ogv|3gp|mp3|m4a|ogg|opus|flac|wav)$/i;
const AUDIO_EXT = /\.(mp3|m4a|ogg|opus|flac|wav)$/;
const RANGE_QUERY = /[?&](range|bytestart|byterange)=/i;

const HLS: Found = { media: "hls", kind: "ytdlp", mime: "application/x-mpegurl", size: 0 };
const DASH: Found = { media: "dash", kind: "ytdlp", mime: "application/dash+xml", size: 0 };

/** Without the query, so the same file fetched twice is one entry. */
export function keyOf(url: URL): string {
  return url.origin + url.pathname;
}

export function webAddress(raw: string): URL | null {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return null;
  }
  return /^https?:$/.test(url.protocol) && !SKIP_HOSTS.test(url.hostname) ? url : null;
}

function header(response: Pick<Response, "responseHeaders">, name: string): string {
  return response.responseHeaders?.find((h) => h.name.toLowerCase() === name)?.value ?? "";
}

function mimeOf(response: Response): string {
  return header(response, "content-type").split(";")[0]!.trim().toLowerCase();
}

export function classify(response: Response): Found | null {
  if (response.tabId < 0 || (response.statusCode !== 200 && response.statusCode !== 206)) {
    return null;
  }
  const url = webAddress(response.url);
  if (!url) return null;
  const path = url.pathname.toLowerCase();
  const mime = mimeOf(response);
  if (/mpegurl/.test(mime) || path.endsWith(".m3u8") || /[./=]m3u8(\b|$)/i.test(url.search)) {
    return HLS;
  }
  if (/dash\+xml/.test(mime) || path.endsWith(".mpd")) return DASH;
  const isMedia = /^(video|audio)\//.test(mime) && !/mp2t|mpegurl/.test(mime);
  const looksMedia = MEDIA_EXT.test(path) && /octet-stream|binary|^$/.test(mime);
  if (!isMedia && !looksMedia) return null;
  if (FRAGMENT.test(path) || RANGE_QUERY.test(url.search)) return null;
  const total = header(response, "content-range").match(/\/(\d+)\s*$/);
  const size = total ? +total[1]! : +header(response, "content-length") || 0;
  if (size && size < 200 * 1024) return null; // ad beacons, previews, sound effects
  const media = /^audio\//.test(mime) || AUDIO_EXT.test(path) ? "audio" : "video";
  return {
    media,
    kind: "http",
    mime: mime || "video/mp4",
    size,
    filename:
      dispositionName(header(response, "content-disposition")) || meaningfulName(response.url),
  };
}

const CAPTION_MIN = 1024;
const CAPTION_MAX = 5 * 1024 * 1024;

/** A complete caption file, not an HLS cue segment. */
export function classifyCaption(response: Response): Found | null {
  if (response.tabId < 0 || (response.statusCode !== 200 && response.statusCode !== 206)) {
    return null;
  }
  const url = webAddress(response.url);
  if (!url) return null;
  if (isYouTubeCaption(url)) {
    // Ask for .srt (YouTube's WebVTT repeats each line as it scrolls) at the same address, which
    // carries the token YouTube requires.
    const srt = new URL(url);
    srt.searchParams.set("fmt", "srt");
    return {
      media: "subtitles",
      kind: "http",
      mime: "application/x-subrip",
      size: 0,
      url: srt.href,
    };
  }
  const path = url.pathname.toLowerCase();
  if (VTT_SEGMENT.test(path)) return null;
  const mime = mimeOf(response);
  if (/^(text\/html|application\/json|application\/javascript|text\/javascript)\b/.test(mime)) {
    return null;
  }
  const name =
    dispositionName(header(response, "content-disposition")) || captionName(url.pathname);
  const looksCaption =
    CAPTION_EXT.test(path) ||
    CAPTION_EXT.test(name) ||
    CAPTION_MIME.test(mime) ||
    captionQuery(url) ||
    (CAPTION_HOST.test(url.hostname) &&
      /download/i.test(path + name) &&
      /octet-stream|text\/plain|^$/.test(mime));
  if (!looksCaption) return null;
  const total = header(response, "content-range").match(/\/(\d+)\s*$/);
  const size = total ? +total[1]! : +header(response, "content-length") || 0;
  if (size && (size < CAPTION_MIN || size > CAPTION_MAX)) return null;
  return {
    media: "subtitles",
    kind: "http",
    mime: mime || "application/x-subrip",
    size,
    filename: name || undefined,
  };
}

function isYouTubeCaption(url: URL): boolean {
  const q = url.searchParams;
  return (
    /(^|\.)youtube\.com$/i.test(url.hostname) &&
    url.pathname === "/api/timedtext" &&
    !!q.get("v") &&
    !!(q.get("lang") || q.get("tlang"))
  );
}

/** YouTube switches videos without a page load, so earlier videos' captions stay listed. */
export function captionForPage(captionUrl: string, page: string): boolean {
  const url = webAddress(captionUrl);
  if (!url || !isYouTubeCaption(url)) return true;
  let id: string | undefined;
  try {
    const p = new URL(page);
    id = p.searchParams.get("v") || /\/(?:shorts|embed|live)\/([\w-]+)/.exec(p.pathname)?.[1];
  } catch {
    return true;
  }
  return !id || id === url.searchParams.get("v");
}

/** /api/v1/captions/ID?label=English and the like: a file, not the language list. */
function captionQuery(url: URL): boolean {
  if (!/\/captions?\b/i.test(url.pathname)) return false;
  const q = url.searchParams;
  return !!(
    q.get("label") ||
    q.get("lang") ||
    q.get("language") ||
    q.get("tlang") ||
    q.get("sub_lang")
  );
}

function captionName(pathname: string): string {
  let name = (pathname.split("/").pop() || "").split("?")[0] || "";
  try {
    name = decodeURIComponent(name);
  } catch {
    // keep it as sent
  }
  return CAPTION_EXT.test(name) ? name : "";
}

/** From the page's own resource list, which may predate the extension watching; no sizes. */
export function classifySeen(entry: SeenEntry): Found | null {
  const url = webAddress(entry.url);
  if (!url) return null;
  const path = url.pathname.toLowerCase();
  if (path.endsWith(".m3u8")) return HLS;
  if (path.endsWith(".mpd")) return DASH;
  if (
    MEDIA_EXT.test(path) &&
    !FRAGMENT.test(path) &&
    /^(video|audio)$/.test(entry.initiator || "") &&
    !RANGE_QUERY.test(url.search)
  ) {
    const media = AUDIO_EXT.test(path) ? "audio" : "video";
    return {
      media,
      kind: "http",
      mime: "",
      size: 0,
      filename: meaningfulName(entry.url) || undefined,
    };
  }
  return null;
}

// Responses that could be a playlist under another name (small, script-fetched, vague type).
// Snag refetches them itself to look at how they start.
const VAGUE_TYPE =
  /^(|text\/plain|application\/octet-stream|binary\/octet-stream|application\/x-binary|application\/binary|application\/x-mpegurl|text\/html)$/;
const NOT_PLAYLIST =
  /\.(js|mjs|css|json|map|png|jpe?g|gif|webp|avif|svg|ico|woff2?|ttf|otf|ts|m4s|mp4|m4a|webm|aac|vtt|srt|ass|ssa|ttml|wasm|html?)$/i;

export function mightBePlaylist(response: Response): boolean {
  if (response.tabId < 0 || response.type !== "xmlhttprequest") return false;
  if (response.statusCode !== 200 && response.statusCode !== 206) return false;
  const url = webAddress(response.url);
  if (!url || NOT_PLAYLIST.test(url.pathname)) return false;
  const length = +header(response, "content-length") || 0;
  return VAGUE_TYPE.test(mimeOf(response)) && length <= 1024 * 1024;
}

export function dispositionName(disposition: string): string {
  let m = /filename\*\s*=\s*[^']*'[^']*'([^;]+)/i.exec(disposition);
  if (m) {
    try {
      return decodeURIComponent(m[1]!.trim().replace(/^"|"$/g, ""));
    } catch {
      // badly encoded: try the plain filename=
    }
  }
  m = /filename\s*=\s*"([^"]+)"/i.exec(disposition) || /filename\s*=\s*([^;]+)/i.exec(disposition);
  if (!m) return "";
  let name = m[1]!.trim();
  if (/%[0-9a-f]{2}/i.test(name)) {
    try {
      name = decodeURIComponent(name); // some servers send it encoded
    } catch {
      // keep it as sent
    }
  }
  return name.split(/[\\/]/).pop() || "";
}

const PLACEHOLDER_NAME =
  /^(index|master|playlist|chunklist|manifest|prog_index|media|video|stream|output|file|download|play|get|main|source|default)([-_. ].*)?$/i;

/** The address's file name, unless it is a placeholder ("index", "master"), hash or digits. */
export function meaningfulName(url: string): string {
  let name: string;
  try {
    name = decodeURIComponent(new URL(url).pathname.split("/").pop() || "");
  } catch {
    return "";
  }
  const stem = name.replace(/\.[a-z0-9]{2,5}$/i, "");
  if (!stem || !/\.[a-z0-9]{2,5}$/i.test(name)) return "";
  if (PLACEHOLDER_NAME.test(stem)) return "";
  const letters = (stem.match(/[a-z]/gi) || []).length;
  const digits = (stem.match(/[0-9]/g) || []).length;
  if (letters < 3 || digits > letters) return ""; // 1434659607842-pgv4ql...
  if (/^[0-9a-f]{6,}$/i.test(stem.replace(/[-_]/g, ""))) return ""; // 7a1f3c9e2b, uuids
  if (/^[0-9a-f-]{12,}$/i.test(stem)) return "";
  if (/[a-z0-9]{24,}/i.test(stem.replace(/[-_. ]/g, "")) && !/[-_. ]/.test(stem)) return "";
  return name;
}

export function readPlaylist(type: StreamType, text: string): PlaylistInfo {
  if (type === "dash") {
    const m = /mediaPresentationDuration="PT(?:(\d+)H)?(?:(\d+)M)?(?:([\d.]+)S)?"/.exec(text);
    const duration = m ? (+m[1]! || 0) * 3600 + (+m[2]! || 0) * 60 + (+m[3]! || 0) : 0;
    return { type: "dash", duration };
  }
  if (/#EXT-X-STREAM-INF/.test(text)) {
    const heights = [...text.matchAll(/RESOLUTION=\d+x(\d+)/g)].map((m) => +m[1]!);
    return {
      type: "master",
      variants: (text.match(/#EXT-X-STREAM-INF/g) || []).length,
      height: Math.max(0, ...heights),
    };
  }
  const durations = [...text.matchAll(/#EXTINF:\s*([\d.]+)/g)].map((m) => +m[1]!);
  const segments = text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith("#"));
  const share = (re: RegExp) =>
    segments.length && segments.filter((l) => re.test(l.split("?")[0]!)).length / segments.length;
  return {
    type:
      share(/\.(vtt|webvtt|srt)$/i) > 0.5
        ? "subtitles"
        : share(/\.(aac|m4a|mp3|ac3|ec3)$/i) > 0.5
          ? "audio"
          : "media",
    duration: Math.round(durations.reduce((a, b) => a + b, 0)),
    live: !/#EXT-X-ENDLIST/.test(text) && !/#EXT-X-PLAYLIST-TYPE:VOD/.test(text),
  };
}

export function extensionOf(url: string, mime = ""): string {
  const m = new URL(url).pathname.match(/\.([a-z0-9]{2,5})$/i);
  if (m) return "." + m[1]!.toLowerCase();
  const byType: Record<string, string> = {
    "video/mp4": ".mp4",
    "video/webm": ".webm",
    "audio/mpeg": ".mp3",
    "audio/mp4": ".m4a",
  };
  return byType[mime] || "";
}
