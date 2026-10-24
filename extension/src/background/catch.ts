// Takes over the browser's downloads of the usual download types and anything big; the browser
// gets them back if Snag can't take one.
import { api } from "../shared/browser";
import { isWebAddress } from "../shared/format";
import { showError } from "./badge";
import { startDownload } from "./page";
import { settings, settingsReady } from "./settings";

// File types taken over whatever their size.
const CATCH_TYPES =
  /\.(3gp|7z|aac|apk|avi|bin|bz2|deb|dmg|exe|flac|flv|gz|img|iso|m4a|m4v|mkv|mov|mp3|mp4|mpeg|mpg|msi|ogg|ogv|opus|pdf|rar|rpm|tar|tgz|wav|webm|wma|wmv|xz|zip|zst|appimage)$/i;

const handedBack = new Set<string>(); // given back to the browser after Snag failed

/**
 * The real name (Content-Disposition) and size arrive a moment after the download starts: wait
 * briefly rather than fall back to the address, which on file hosts is a token.
 */
async function settled(
  item: chrome.downloads.DownloadItem,
): Promise<chrome.downloads.DownloadItem> {
  for (
    let waited = 0;
    waited < 2500 && (!item.filename || !(item.totalBytes > 0)) && item.state === "in_progress";
    waited += 150
  ) {
    await new Promise((resolve) => setTimeout(resolve, 150));
    const [now] = await api.downloads.search({ id: item.id }).catch(() => []);
    if (!now) break;
    item = now;
  }
  return item;
}

async function take(item: chrome.downloads.DownloadItem): Promise<void> {
  await settingsReady;
  const first = item.finalUrl || item.url;
  if (!settings.intercept || item.byExtensionId === api.runtime.id) return;
  if (!isWebAddress(first) || handedBack.has(first)) return;
  item = await settled(item);
  const url = item.finalUrl || item.url;
  const name = item.filename || new URL(url).pathname;
  const big = item.totalBytes > 0 && item.totalBytes >= settings.minSize;
  if (!CATCH_TYPES.test(name) && !big) return;
  try {
    await api.downloads.cancel(item.id);
    await api.downloads.erase({ id: item.id });
  } catch {
    return;
  }
  try {
    const size = item.totalBytes > 0 ? item.totalBytes : item.fileSize > 0 ? item.fileSize : 0;
    const filename = item.filename ? item.filename.split(/[\\/]/).pop() : "";
    // Browsers don't say which tab a download came from; the referrer stands in for the page.
    await startDownload({ url, kind: "http", filename, size }, null, item.referrer);
  } catch (error) {
    handedBack.add(url);
    setTimeout(() => handedBack.delete(url), 60_000);
    api.downloads.download({ url }).catch(() => {});
    showError(error);
  }
}

export function catchDownloads(): void {
  api.downloads.onCreated.addListener((item) => void take(item));
}
