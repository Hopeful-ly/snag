// The feeds a page links to with <link rel="alternate">, where podcast sites list their episodes.

const FEED_TYPES = /^application\/(rss|atom)\+xml\b/i;
const MAX_FEEDS = 6;

export function pageFeeds(doc: Document): string[] {
  const out: string[] = [];
  for (const link of doc.querySelectorAll<HTMLLinkElement>("link[rel][href][type]")) {
    const rel = link.rel.toLowerCase().split(/\s+/);
    if (!rel.includes("alternate") || !FEED_TYPES.test(link.type.trim())) continue;
    let url: URL;
    try {
      url = new URL(link.getAttribute("href") || "", doc.baseURI);
    } catch {
      continue;
    }
    if (url.protocol !== "http:" && url.protocol !== "https:") continue;
    url.hash = "";
    if (!out.includes(url.href)) out.push(url.href);
    if (out.length === MAX_FEEDS) break;
  }
  return out;
}
