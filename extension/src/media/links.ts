// Runs inside the page (in every frame) via executeScript, so it must stand alone: no imports.

export interface FoundLink {
  url: string;
  text?: string;
  name?: string;
}

export function collectLinks(selectionOnly: boolean): FoundLink[] {
  const selection = selectionOnly ? document.getSelection() : null;
  const ranges: Range[] = [];
  if (selection)
    for (let i = 0; i < selection.rangeCount; i++) ranges.push(selection.getRangeAt(i));
  if (selectionOnly && !ranges.some((r) => !r.collapsed)) return [];
  const chosen = (el: Element) => !selectionOnly || ranges.some((r) => r.intersectsNode(el));

  const found: FoundLink[] = [];
  const seen = new Map<string, FoundLink>();
  const add = (url: string | null | undefined, text?: string, name?: string) => {
    if (!url || !/^https?:/i.test(url)) return;
    const words = (text || "").replace(/\s+/g, " ").trim().slice(0, 200);
    const known = seen.get(url);
    if (known) {
      if (!known.text && words) known.text = words;
      return;
    }
    const link: FoundLink = { url };
    if (words) link.text = words;
    if (name) link.name = name;
    seen.set(url, link);
    found.push(link);
  };

  for (const a of document.querySelectorAll<HTMLAnchorElement | HTMLAreaElement>(
    "a[href], area[href]",
  )) {
    if (!chosen(a)) continue;
    const text = a instanceof HTMLAnchorElement ? a.textContent || a.title : a.alt;
    add(a.href, text || a.querySelector("img")?.alt || "", a.getAttribute("download") || "");
  }
  for (const img of document.querySelectorAll<HTMLImageElement>("img")) {
    if (chosen(img)) add(img.currentSrc || img.src, img.alt);
  }
  for (const media of document.querySelectorAll<HTMLMediaElement>("video, audio")) {
    if (!chosen(media)) continue;
    add(media.currentSrc || media.src);
    for (const source of media.querySelectorAll("source")) add(source.src);
  }
  return found.slice(0, 5000);
}
