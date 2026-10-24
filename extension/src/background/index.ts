import { classify, classifyCaption, keyOf, mightBePlaylist } from "../media/detect";
import { isSpotify } from "../media/spotify";
import { api } from "../shared/browser";
import type { Answer, Command } from "../shared/protocol";
import { catchDownloads } from "./catch";
import {
  captionFilesOf,
  forget,
  foundReady,
  mediaOf,
  remember,
  rememberCaption,
  rememberSeen,
} from "./found";
import { onMessage, send } from "./link";
import { addMenus, refreshOpenPages } from "./menus";
import { overlayVisible, setOverlayVisible } from "./overlay";
import {
  feedsOf,
  lookupHave,
  openPlaylist,
  pageTitleOf,
  probe,
  probePage,
  startDownload,
  type Tab,
} from "./page";
import "./activity";
import { settings, settingsReady } from "./settings";
import { resolvePage } from "./resolve";
import { clearShown, frameRequested } from "./shown";
import { forgetSpotify, watchSpotify } from "./spotify";
import { forgetSniffed, sniff } from "./sniff";
import { forgetTMDB, noteRequest, tmdbOf } from "./tmdb";

api.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.tabId < 0) return;
    forgetSniffed(details.tabId);
    forget(details.tabId);
    forgetTMDB(details.tabId);
    if (!isSpotify(details.url)) forgetSpotify(details.tabId); // a Spotify page keeps its own
  },
  { urls: ["<all_urls>"], types: ["main_frame"] },
);

// A frame loading in a tab Snag opened to find a stream may hold the player.
api.webRequest.onBeforeRequest.addListener(
  (details) => {
    frameRequested(details.tabId, details.url);
  },
  { urls: ["<all_urls>"], types: ["sub_frame"] },
);

api.webRequest.onHeadersReceived.addListener(
  (details) => {
    noteRequest(details.tabId, details.url);
    const found = classify(details);
    if (found) {
      const key = keyOf(new URL(details.url));
      const frame = details.frameId || 0;
      const referer = refererOf(details);
      remember(details.tabId, {
        key,
        url: details.url,
        ...found,
        time: Date.now(),
        frame,
        referer,
      });
      // Read a playlist to learn what it is (subtitles, an ad, its length).
      if (found.media === "hls" || found.media === "dash") sniff(details, true);
      return;
    }
    const caption = classifyCaption(details);
    if (caption) {
      rememberCaption(details.tabId, {
        key: keyOf(new URL(details.url)),
        url: details.url,
        ...caption,
        time: Date.now(),
        frame: details.frameId || 0,
      });
      return;
    }
    if (mightBePlaylist(details)) sniff(details, false);
  },
  {
    urls: ["<all_urls>"],
    types: ["media", "xmlhttprequest", "object", "other", "main_frame", "sub_frame"],
  },
  ["responseHeaders"],
);

api.tabs.onRemoved.addListener((tabId) => {
  forget(tabId);
  forgetTMDB(tabId);
  forgetSpotify(tabId);
});

watchSpotify();

/** The page or frame that made a request: Firefox names it, Chrome gives its origin. */
function refererOf(details: chrome.webRequest.WebRequestDetails): string | undefined {
  const firefox = (details as { documentUrl?: string }).documentUrl;
  if (firefox) return firefox;
  return details.initiator && details.initiator !== "null" ? details.initiator + "/" : undefined;
}

void clearShown();
onMessage((message) => {
  if (message.type === "resolve") void resolvePage(message.token, message.page);
});

api.runtime.onMessage.addListener((message: Command, sender, respond) => {
  answer(message, sender).then(
    (value) => respond({ ok: true, value } satisfies Answer),
    (error) => respond({ ok: false, error: String(error?.message || error) } satisfies Answer),
  );
  return true; // the answer comes later
});

async function tabFor(
  message: Command,
  sender: chrome.runtime.MessageSender,
): Promise<Tab | undefined> {
  if (message.tabId != null) return api.tabs.get(message.tabId);
  if (sender.tab) return sender.tab;
  const [tab] = await api.tabs.query({ active: true, currentWindow: true });
  return tab;
}

async function answer(message: Command, sender: chrome.runtime.MessageSender): Promise<unknown> {
  await Promise.all([settingsReady, foundReady]);
  const tab = await tabFor(message, sender);
  switch (message.cmd) {
    case "settings":
      return { ...settings, overlay: await overlayVisible() };
    case "overlay":
      if (message.visible != null) return { visible: await setOverlayVisible(message.visible) };
      return { visible: await overlayVisible() };
    case "seen":
      if (Array.isArray(message.entries) && tab?.id != null) rememberSeen(tab.id, message.entries);
      return { media: mediaOf(tab?.id), captions: captionFilesOf(tab?.id) };
    case "media":
      return mediaOf(tab?.id);
    case "probe":
      return probePage(tab, message.frameUrl || sender.url);
    case "probeStream":
      return probe(message.url, tab, true);
    case "download":
      return startDownload(message.item, tab, message.referer || sender.url);
    case "have":
      return lookupHave(message.items, tab);
    case "control":
      return send({ type: message.action, id: message.id, delete_file: !!message.deleteFile });
    case "pageTitle":
      return pageTitleOf(tab);
    case "show":
      return send({ type: "show", id: message.id || "" });
    case "playlist": {
      // Only asking whether there is a list: worth it for a page that read a show from TMDB, or
      // links to feeds that may be a podcast's (the page check already listed what yt-dlp can).
      const tmdb = message.tmdb || (await tmdbOf(tab?.id));
      const feeds = await feedsOf(tab);
      const onlyFeeds = message.open === false && !tmdb;
      if (onlyFeeds && !feeds.length) return null;
      return openPlaylist(
        [message.url, tab?.url],
        tab,
        message.open ?? true,
        tmdb,
        !!message.subscribe,
        feeds,
        onlyFeeds,
      );
    }
    case "status":
      try {
        await send({ type: "ping" });
        return { ok: true };
      } catch (error) {
        return { ok: false, error: String((error as Error).message || error) };
      }
  }
  throw new Error("unknown command");
}

addMenus();
refreshOpenPages();
catchDownloads();
