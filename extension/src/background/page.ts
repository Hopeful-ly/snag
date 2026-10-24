// Talks to Snag about pages with the referer, user agent and cookies sites expect from the browser.
import { extensionOf } from "../media/detect";
import { collectLinks, type FoundLink } from "../media/links";
import { isSpotifyList } from "../media/spotify";
import { api } from "../shared/browser";
import { isWebAddress } from "../shared/format";
import type { PageChecksums } from "../media/checksum";
import type {
  Added,
  AddRequest,
  ChecksumQuestion,
  Cookie,
  Credentials,
  Download,
  FeedsQuestion,
  HaveItem,
  Item,
  PlaylistFound,
  ProbeResult,
  TitleQuestion,
} from "../shared/protocol";
import { send } from "./link";
import { spotifyOf } from "./spotify";
import { tmdbOf } from "./tmdb";

/** cookieStoreId: Firefox containers keep their cookies apart. */
export type Tab = chrome.tabs.Tab & { cookieStoreId?: string };

async function cookiesFor(url: string, storeId?: string): Promise<Cookie[]> {
  if (!api.cookies) return [];
  try {
    const list = await api.cookies.getAll(storeId ? { url, storeId } : { url });
    return list.map(({ domain, name, value, path, secure, expirationDate, hostOnly }) => ({
      domain,
      name,
      value,
      path,
      secure,
      expirationDate,
      hostOnly,
    }));
  } catch {
    return [];
  }
}

export async function credentials(
  url: string,
  referer: string | undefined,
  tab: Tab | null | undefined,
): Promise<Credentials> {
  const headers: Record<string, string> = { "User-Agent": navigator.userAgent };
  if (isWebAddress(referer)) headers.Referer = referer;
  return { headers, cookies: await cookiesFor(url, tab?.cookieStoreId) };
}

// Probe results are shared for a few minutes (the card and other frames ask again).
const probes = new Map<string, { time: number; promise: Promise<ProbeResult> }>();
const PROBE_FOR = 5 * 60 * 1000;

export function probe(
  url: string,
  tab: Tab | null | undefined,
  stream = false,
): Promise<ProbeResult> {
  if (!isWebAddress(url)) return Promise.resolve({ supported: false });
  const cached = probes.get(url);
  if (cached && Date.now() - cached.time < PROBE_FOR) return cached.promise;
  const promise = (async () => {
    const extra = await credentials(url, tab?.url, tab);
    return (await send({ type: "probe", url, stream, ...extra }, 120_000)).probe;
  })();
  probes.set(url, { time: Date.now(), promise });
  promise.catch(() => probes.delete(url));
  if (probes.size > 100) probes.delete(probes.keys().next().value!);
  return promise;
}

/** Probes the video's frame (embedded players) and the page at once; the frame's answer wins. */
export async function probePage(
  tab: Tab | undefined,
  frameUrl: string | undefined,
): Promise<ProbeResult> {
  const urls = [...new Set([frameUrl, tab?.url].filter(isWebAddress))];
  const answers = urls.map((url) => probe(url, tab));
  let last: ProbeResult = { supported: false };
  let failure: unknown;
  for (const answer of answers) {
    try {
      last = await answer;
    } catch (error) {
      failure ??= error;
      continue;
    }
    if (last.supported) return last;
  }
  if (failure && !last.reason) throw failure;
  return last;
}

/**
 * open=false only checks whether there is a list; feedsOnly means the page itself was already
 * checked.
 */
export async function openPlaylist(
  urls: (string | undefined)[],
  tab: Tab | null | undefined,
  open = true,
  tmdbCopy = "",
  subscribe = false,
  pageFeeds?: string[],
  feedsOnly = false,
): Promise<PlaylistFound | null> {
  const tmdb = tmdbCopy || (await tmdbOf(tab?.id));
  const feeds = pageFeeds ?? (await feedsOf(tab));
  for (const url of new Set(urls.filter(isWebAddress))) {
    const extra = await credentials(url, tab?.url, tab);
    const spotify = isSpotifyList(url) ? await spotifyOf(tab?.id) : undefined;
    // Feeds belong to the tab's page, not to links on it.
    const ofPage = url === tab?.url ? feeds : undefined;
    if (feedsOnly && !ofPage?.length) continue;
    const { playlist } = await send(
      {
        type: "playlist",
        url,
        title: tab?.title || "",
        tmdb,
        spotify,
        open,
        subscribe,
        feeds: ofPage,
        feeds_only: feedsOnly || undefined,
        ...extra,
      },
      120_000,
    );
    if (playlist) return playlist;
  }
  return null;
}

/** Asked of the top frame: an embedded player's frame knows nothing of the show. */
export async function pageTitleOf(tab: Tab | null | undefined): Promise<string> {
  const fallback = (tab?.title || "").replace(/\s+[-|–]\s+[^-|–]{1,40}$/, "").trim(); // " - Site"
  if (tab?.id == null) return fallback;
  try {
    const question: TitleQuestion = { cmd: "title" };
    const reply = await Promise.race([
      Promise.resolve(api.tabs.sendMessage(tab.id, question, { frameId: 0 })),
      new Promise((resolve) => setTimeout(() => resolve(null), 800)),
    ]);
    return (reply as { title?: string } | null)?.title || fallback;
  } catch {
    return fallback;
  }
}

export async function feedsOf(tab: Tab | null | undefined): Promise<string[]> {
  if (tab?.id == null) return [];
  try {
    const question: FeedsQuestion = { cmd: "feeds" };
    const reply = await Promise.race([
      Promise.resolve(api.tabs.sendMessage(tab.id, question, { frameId: 0 })),
      new Promise((resolve) => setTimeout(() => resolve(null), 800)),
    ]);
    return Array.isArray(reply) ? reply.filter(isWebAddress) : [];
  } catch {
    return []; // a page the extension can't reach
  }
}

export async function lookupHave(
  items: HaveItem[],
  tab: Tab | null | undefined,
): Promise<(Download | null)[]> {
  if (!items.length) return [];
  const reply = await send({
    type: "have",
    page: tab?.url || "",
    title: await pageTitleOf(tab),
    items,
  });
  return reply.have;
}

/** Without a tab (the browser's own downloads), asks the open tab at referer. */
async function checksumsOf(
  url: string,
  tab: Tab | null | undefined,
  referer?: string,
): Promise<PageChecksums | null> {
  try {
    let id = tab?.id;
    if (id == null && isWebAddress(referer)) {
      const tabs = await api.tabs.query({});
      id = tabs.find((t) => t.url === referer)?.id;
    }
    if (id == null) return null;
    const question: ChecksumQuestion = { cmd: "checksums", url };
    const reply = await Promise.race([
      Promise.resolve(api.tabs.sendMessage(id, question)),
      new Promise((resolve) => setTimeout(() => resolve(null), 800)),
    ]);
    return (reply as PageChecksums | null) ?? null;
  } catch {
    return null; // a page the extension can't reach
  }
}

export async function startDownload(
  item: Item,
  tab: Tab | null | undefined,
  referer?: string,
): Promise<Added> {
  const page = tab?.url || "";
  const stream = item.media === "hls" || item.media === "dash";
  // A site yt-dlp knows names its videos itself; streams and files get the page's name.
  const fromSite = item.kind === "ytdlp" && !stream;
  const title = fromSite && item.title ? item.title : await pageTitleOf(tab);
  const request: AddRequest = {
    type: "add",
    url: item.url,
    kind: item.kind,
    page,
    title,
    label: item.label || "",
    size: Math.round(item.size || 0),
    ...(await credentials(item.url, referer || page, tab)),
  };
  if (item.kind === "ytdlp") {
    if (item.format) request.format = item.format;
    if (item.sort) request.sort = item.sort;
    if (item.audio) request.audio = item.audio;
    if (title && stream) request.filename = title; // Snag adds the extension
  } else if (title && !item.filename && item.media && item.media !== "subtitles") {
    request.filename = title + extensionOf(item.url, item.mime);
  }
  if (item.subs && !item.audio) {
    request.subs = item.subs;
    if (item.sub_auto) request.sub_auto = true;
    if (item.sidecar) request.sidecar = true;
    if (item.sub_url) {
      request.sub_url = item.sub_url;
      const extra = await credentials(item.sub_url, referer || page, tab);
      request.cookies = [...request.cookies, ...extra.cookies];
    }
    if (item.sub_text) request.sub_text = item.sub_text;
  }
  if (item.filename) request.filename = item.filename;
  if (item.clip_to) {
    request.clip_from = item.clip_from || 0;
    request.clip_to = item.clip_to;
  }
  if (item.kind === "http" && !item.media) {
    const sums = await checksumsOf(item.url, tab, referer || page);
    if (sums?.checksum) request.checksum = sums.checksum;
    if (sums?.files.length) request.checksum_files = sums.files;
  }
  const reply = await send(request);
  return { ...reply.download, duplicate: !!reply.duplicate };
}

export async function openLinks(
  tab: Tab | null | undefined,
  selectionOnly: boolean,
): Promise<number> {
  if (tab?.id == null) throw new Error("Snag can't read this page");
  const found = await linksInTab(tab.id, selectionOnly);
  if (!found.length)
    throw new Error(
      selectionOnly ? "There are no links in what is selected" : "This page has no links",
    );
  // Cookies for every site the links are on (CDNs, mirrors), not just the page's.
  const hosts = new Set<string>();
  for (const l of found) {
    try {
      hosts.add(new URL(l.url).origin);
    } catch {
      // not an address
    }
    if (hosts.size >= 20) break;
  }
  const page = tab.url || "";
  const pageOrigin = isWebAddress(page) ? new URL(page).origin : "";
  const extra = await credentials(page, page, tab);
  for (const origin of hosts) {
    if (origin === pageOrigin) continue;
    extra.cookies.push(...(await credentials(origin + "/", page, tab)).cookies);
  }
  const { links } = await send(
    {
      type: "links",
      page,
      title: tab.title || "",
      source: selectionOnly ? "selection" : "page",
      links: found,
      ...extra,
    },
    60_000,
  );
  return links.count;
}

async function linksInTab(tabId: number, selectionOnly: boolean): Promise<FoundLink[]> {
  let results: unknown[];
  if (api.scripting) {
    const frames = await api.scripting.executeScript({
      target: { tabId, allFrames: true },
      func: collectLinks,
      args: [selectionOnly],
    });
    results = frames.map((f) => f.result);
  } else {
    // Firefox's Manifest V2 runs code given as text; the function is whole in itself.
    const legacy = api.tabs as unknown as {
      executeScript(id: number, details: { code: string; allFrames: boolean }): Promise<unknown[]>;
    };
    results = await legacy.executeScript(tabId, {
      code: `(${collectLinks.toString()})(${JSON.stringify(selectionOnly)})`,
      allFrames: true,
    });
  }
  const seen = new Set<string>();
  const all: FoundLink[] = [];
  for (const frame of results) {
    if (!Array.isArray(frame)) continue;
    for (const link of frame as FoundLink[]) {
      if (link && typeof link.url === "string" && !seen.has(link.url)) {
        seen.add(link.url);
        all.push(link);
      }
    }
  }
  return all;
}
