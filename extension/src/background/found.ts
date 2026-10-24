// The media each tab loaded, in session storage so it survives Chrome stopping an idle background.
import { classifySeen, keyOf, readPlaylist, webAddress } from "../media/detect";
import { api } from "../shared/browser";
import type { Media, SeenEntry, StreamType } from "../shared/protocol";
import { setCount } from "./badge";

const MAX_PER_TAB = 40;
const MAX_CAPTIONS = 30;

const found = new Map<number, Media[]>();
const captions = new Map<number, Media[]>();
const session = api.storage.session;
let saveTimer: ReturnType<typeof setTimeout> | undefined;

function save(): void {
  if (!session) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(
    () =>
      session
        .set({ found: Object.fromEntries(found), captions: Object.fromEntries(captions) })
        .catch(() => {}),
    500,
  );
}

export const foundReady: Promise<void> = session
  ? session
      .get(["found", "captions"])
      .then((saved) => {
        const lists = (saved.found || {}) as Record<string, Media[]>;
        let dirty = false;
        for (const [tab, list] of Object.entries(lists)) {
          if (found.has(+tab)) continue;
          const clean = list.filter((e) => e.media !== "subtitles");
          if (clean.length !== list.length) dirty = true;
          found.set(+tab, clean);
        }
        const fetched = (saved.captions || {}) as Record<string, Media[]>;
        for (const [tab, list] of Object.entries(fetched)) {
          if (captions.has(+tab)) continue;
          captions.set(
            +tab,
            list.filter((e) => e.media === "subtitles"),
          );
        }
        if (dirty) save();
      })
      .catch(() => {})
  : Promise.resolve();

function update(tabId: number, list: Media[]): void {
  found.set(tabId, list);
  save();
  setCount(tabId, list.length);
}

export function mediaOf(tabId: number | undefined): Media[] {
  if (tabId == null) return [];
  return [...(found.get(tabId) || [])].sort((a, b) => a.time - b.time);
}

/** Oldest first, so a later language file replaces an earlier one. */
export function captionFilesOf(tabId: number | undefined): Media[] {
  if (tabId == null) return [];
  return [...(captions.get(tabId) || [])].sort((a, b) => a.time - b.time);
}

export function has(tabId: number, key: string): boolean {
  return (found.get(tabId) || []).some((e) => e.key === key);
}

export function remember(tabId: number, entry: Media): void {
  if (entry.media === "subtitles") {
    rememberCaption(tabId, entry);
    return;
  }
  const list = found.get(tabId) || [];
  const i = list.findIndex((e) => e.key === entry.key);
  const known = list[i];
  if (known) {
    entry.size = entry.size || known.size;
    entry.info = entry.info || known.info;
    entry.time = known.time; // keep first-seen order (masters come before variants)
    list[i] = entry;
  } else {
    list.push(entry);
    if (list.length > MAX_PER_TAB) list.shift();
  }
  update(tabId, list);
}

/** Kept apart from video so captions cannot crowd it out. */
export function rememberCaption(tabId: number, entry: Media): void {
  const list = captions.get(tabId) || [];
  const i = list.findIndex((e) => e.key === entry.key);
  const known = list[i];
  if (known) {
    entry.size = entry.size || known.size;
    entry.time = Date.now(); // same address fetched again is the current choice
    list[i] = entry;
  } else {
    list.push(entry);
    if (list.length > MAX_CAPTIONS) list.shift();
  }
  captions.set(tabId, list);
  save();
}

export function rememberPlaylist(tabId: number, url: string, type: StreamType, text: string): void {
  const address = webAddress(url);
  if (!address) return;
  const info = readPlaylist(type, text);
  const key = keyOf(address);
  const list = found.get(tabId) || [];
  const known = list.find((e) => e.key === key);
  if (info.type === "subtitles") {
    if (known) update(tabId, without(list, known));
    return;
  }
  if (known) {
    known.info = info;
    save();
    return;
  }
  const mime = type === "hls" ? "application/x-mpegurl" : "application/dash+xml";
  remember(tabId, {
    key,
    url,
    media: type,
    kind: "ytdlp",
    mime,
    size: 0,
    time: Date.now(),
    frame: 0,
    info,
  });
}

/** The page's own resource list still names what it fetched before the extension was watching. */
export function rememberSeen(tabId: number, entries: SeenEntry[]): void {
  for (const entry of entries.slice(0, 300)) {
    try {
      const url = webAddress(entry.url);
      if (!url || has(tabId, keyOf(url))) continue;
      const what = classifySeen(entry);
      if (!what) continue;
      remember(tabId, { key: keyOf(url), url: entry.url, ...what, time: Date.now(), frame: 0 });
    } catch {
      // a bad address in the page's list must not wipe the rest
    }
  }
}

const without = (list: Media[], entry: Media) => list.filter((e) => e !== entry);

export function forget(tabId: number): void {
  const had = found.delete(tabId);
  const hadCaptions = captions.delete(tabId);
  if (!had && !hadCaptions) return;
  save();
  setCount(tabId, 0);
}
