// The right-click menu: download a link, a video or audio element, or the page's video. A page
import { isAppleList } from "../media/apple";
import { isSpotifyList } from "../media/spotify";
import { api } from "../shared/browser";
import { isWebAddress } from "../shared/format";
import { showError } from "./badge";
import { send } from "./link";
import { feedsOf, openLinks, openPlaylist, startDownload } from "./page";
import { tmdbOf } from "./tmdb";

/** The context menu API: `contextMenus` in Chrome, `menus` in Firefox. */
const menus: typeof chrome.contextMenus =
  api.contextMenus ?? (api as unknown as { menus: typeof chrome.contextMenus }).menus;

// The best quality yt-dlp offers, in formats every player opens.
const BEST_FORMAT = "bv*+ba/b";
const COMPATIBLE_SORT = "vcodec:h264,acodec:aac,ext:mp4:m4a";

export function addMenus(): void {
  // Made again on every start: Firefox doesn't keep them, and removing them first avoids twins.
  Promise.resolve(menus.removeAll())
    .then(() => {
      menus.create({ id: "snag-link", title: "Download link with Snag", contexts: ["link"] });
      menus.create({
        id: "snag-media",
        title: "Download this media with Snag",
        contexts: ["video", "audio"],
      });
      menus.create({
        id: "snag-page",
        title: "Download from this page with Snag",
        contexts: ["page"],
      });
      menus.create({
        id: "snag-all-links",
        title: "Download all links with Snag…",
        contexts: ["page"],
      });
      menus.create({
        id: "snag-selected-links",
        title: "Download selected links with Snag…",
        contexts: ["selection"],
      });
    })
    .catch(() => {});

  menus.onClicked.addListener(async (info, tab) => {
    try {
      if (info.menuItemId === "snag-all-links" || info.menuItemId === "snag-selected-links") {
        await openLinks(tab, info.menuItemId === "snag-selected-links");
        return;
      }
      let url =
        info.menuItemId === "snag-link"
          ? info.linkUrl
          : info.menuItemId === "snag-media"
            ? info.srcUrl
            : info.pageUrl;
      if (!isWebAddress(url)) url = info.frameUrl || info.pageUrl; // blob: players
      if (!url) return;
      // Spotify's and Apple Music's songs come from YouTube Music, chosen in Snag's playlist
      // window, even a song alone: its page, or a link to one.
      if (info.menuItemId !== "snag-media" && (isSpotifyList(url) || isAppleList(url))) {
        if (!(await openPlaylist([url], tab)))
          throw new Error("Snag found nothing to download there");
        return;
      }
      const { kind } = await send({ type: "kind", url });
      const stream = kind === "ytdlp";
      // A page that is a list, or is in one: yt-dlp's sites, streaming sites' seasons, and
      // podcasts' sites, which link to their feeds.
      const feeds = info.menuItemId === "snag-page" ? await feedsOf(tab) : [];
      const listed = stream || !!(await tmdbOf(tab?.id)) || feeds.length > 0;
      if (
        info.menuItemId === "snag-page" &&
        listed &&
        (await openPlaylist([url], tab, true, "", false, feeds))
      )
        return;
      await startDownload(
        { url, kind, format: stream ? BEST_FORMAT : "", sort: stream ? COMPATIBLE_SORT : "" },
        tab,
        info.frameUrl || info.pageUrl,
      );
    } catch (error) {
      showError(error);
    }
  });
}

/** Open pages run the old content script (or none) until reloaded, so inject the new one. */
export function refreshOpenPages(): void {
  api.runtime.onInstalled.addListener(async () => {
    const files = ["content.js"];
    let tabs: chrome.tabs.Tab[];
    try {
      tabs = await api.tabs.query({ url: ["http://*/*", "https://*/*"] });
    } catch {
      return;
    }
    const legacy = api.tabs as unknown as {
      executeScript(tabId: number, details: { file: string; allFrames: boolean }): Promise<unknown>;
    };
    for (const tab of tabs) {
      if (tab.id == null) continue;
      try {
        if (api.scripting) {
          await api.scripting.executeScript({ target: { tabId: tab.id, allFrames: true }, files });
        } else {
          for (const file of files) await legacy.executeScript(tab.id, { file, allFrames: true });
        }
      } catch {
        // pages extensions may not touch (stores, error pages)
      }
    }
  });
}
