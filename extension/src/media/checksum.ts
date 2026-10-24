// Checksums a page shows for its downloads: a hash next to the link, or links to checksum files
// (SHA256SUMS, file.iso.sha256). Snag verifies only when the user asks.

export interface PageChecksums {
  checksum?: string;
  files: string[];
}

// Hex digits of the lengths checksums have: MD5, SHA-1, SHA-256, SHA-384, SHA-512.
const LENGTHS = new Set([32, 40, 64, 96, 128]);
const HEX = /(?<![0-9a-z])[0-9a-f]{32,128}(?![0-9a-z])/gi;
const NAMED =
  /\b(md5|sha-?1|sha-?256|sha-?384|sha-?512)(?:sum)?\b(?:\s*(?:checksum|hash|digest|sum))?\s*[:=]?\s*$/i;
const CHECKSUM_FILE =
  /(?:\.(?:sha(?:1|256|384|512)(?:sum)?|md5(?:sum)?)|\/(?:sha(?:1|256|512)sums?|md5sums?|checksums?)(?:\.txt)?)$/i;

// How far up from the link to look, and how much text one place may have: a page's whole body
// has hashes of every file on it.
const LEVELS = 3;
const MAX_TEXT = 4000;
const MAX_FILES = 4;

/** The hashes in text, each with the name written just before it, if any; none from skip. */
export function hashesIn(text: string, skip = ""): string[] {
  const out: string[] = [];
  const lowerSkip = skip.toLowerCase();
  for (const m of text.matchAll(HEX)) {
    const hex = m[0].toLowerCase();
    if (!LENGTHS.has(hex.length) || lowerSkip.includes(hex)) continue;
    if (/^(\d+|[a-f]+)$/.test(hex)) continue; // digits or letters alone: an id or a word
    const named = NAMED.exec(text.slice(Math.max(0, (m.index ?? 0) - 30), m.index));
    const label = named?.[1] ? named[1].toUpperCase().replace("-", "") + ": " : "";
    if (!out.some((h) => h.endsWith(hex))) out.push(label + hex);
  }
  return out;
}

// The strongest of several hashes for one file (SHA-512 over SHA-256 over MD5); nothing when
// two of the same kind leave it unclear which is this file's.
function strongest(hashes: string[]): string | undefined {
  const bare = (h: string) => h.replace(/^.*: /, "");
  const longest = Math.max(...hashes.map((h) => bare(h).length));
  const best = hashes.filter((h) => bare(h).length === longest);
  return best.length === 1 ? best[0] : undefined;
}

function sameAddress(a: string, b: string): boolean {
  const plain = (u: string) => {
    try {
      const url = new URL(u);
      url.hash = "";
      return decodeURI(url.href);
    } catch {
      return u;
    }
  };
  return plain(a) === plain(b);
}

export function linksTo(doc: Document, url: string): HTMLAnchorElement[] {
  return [...doc.querySelectorAll<HTMLAnchorElement>("a[href]")].filter((a) =>
    sameAddress(a.href, url),
  );
}

// The text of el, its parts apart: textContent runs cells and lines together ("12 MB3f0b…").
function textOf(el: Element): string {
  const parts: string[] = [];
  let length = 0;
  const walker = el.ownerDocument.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node && length <= MAX_TEXT; node = walker.nextNode()) {
    const text = node.nodeValue?.trim();
    if (text) {
      parts.push(text);
      length += text.length + 1;
    }
  }
  return parts.join(" ");
}

/** A hash shown next to a link: in the link's own attributes, its row, or the block around it. */
function checksumNear(link: Element, url: string): string | undefined {
  for (const attr of ["data-sha256", "data-checksum", "data-hash", "title"]) {
    const found = hashesIn(link.getAttribute(attr) || "", url);
    if (found.length) return strongest(found);
  }
  const places: Element[] = [];
  const row = link.closest("tr, li, dd, dt");
  if (row) places.push(row);
  for (let el = link.parentElement, n = 0; el && n < LEVELS; el = el.parentElement, n++) {
    if (!places.includes(el) && !(row && el.contains(row) && el !== row)) places.push(el);
  }
  // A table's row may hold the hash in the next row ("SHA256: …" under each file).
  if (row?.tagName === "TR" && row.nextElementSibling) places.splice(1, 0, row.nextElementSibling);
  for (const place of places) {
    const text = textOf(place);
    if (text.length > MAX_TEXT) break;
    const found = hashesIn(text, url);
    // The nearest place with hashes decides: wider ones only hold more files' hashes.
    if (found.length) return strongest(found);
  }
  return undefined;
}

export function checksumFiles(doc: Document, url: string): string[] {
  const file = (() => {
    try {
      return decodeURIComponent(new URL(url).pathname.split("/").pop() || "").toLowerCase();
    } catch {
      return "";
    }
  })();
  const found = new Set<string>();
  for (const a of doc.querySelectorAll<HTMLAnchorElement>("a[href]")) {
    let path: string;
    try {
      const u = new URL(a.href);
      if (u.protocol !== "https:" && u.protocol !== "http:") continue;
      path = decodeURIComponent(u.pathname).toLowerCase();
    } catch {
      continue;
    }
    if (CHECKSUM_FILE.test(path)) found.add(a.href);
  }
  const named = [...found].sort((a, b) => {
    const mine = (h: string) =>
      file && decodeURIComponent(h).toLowerCase().includes(file) ? 0 : 1;
    return mine(a) - mine(b);
  });
  return named.slice(0, MAX_FILES);
}

/** null when the page doesn't link to url at all. */
export function pageChecksums(doc: Document, url: string): PageChecksums | null {
  const links = linksTo(doc, url);
  const files = checksumFiles(doc, url);
  if (!links.length) return files.length ? { files } : null;
  for (const link of links) {
    const checksum = checksumNear(link, url);
    if (checksum) return { checksum, files };
  }
  return { files };
}
