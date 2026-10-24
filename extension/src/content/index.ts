import { isAppleMusic } from "../media/apple";
import { linksTo, pageChecksums } from "../media/checksum";
import { pageFeeds } from "../media/feeds";
import { isSpotify } from "../media/spotify";
import { read } from "../media/title";
import { api } from "../shared/browser";
import type { ChecksumQuestion, FeedsQuestion, TitleQuestion } from "../shared/protocol";
import { AppleButton } from "./apple-button";
import { Overlay } from "./overlay";
import { SpotifyButton } from "./spotify-button";

declare global {
  interface Window {
    __snagOverlay?: boolean;
  }
}

if (!window.__snagOverlay) {
  window.__snagOverlay = true;

  // Keep a longer list of what the page fetched: the default 250 fill fast on busy sites, and a
  // stream's playlist is among the first (see the card's seenEntries).
  try {
    performance.setResourceTimingBufferSize(3000);
  } catch {
    // not allowed here
  }
  // A copy of this script from before an extension update may have left its button behind.
  for (const old of document.querySelectorAll("snag-overlay, snag-spotify, snag-apple"))
    old.remove();

  // The top page names what is playing and lists its feeds (for podcasts).
  if (window === window.top) {
    try {
      api.runtime.onMessage.addListener(
        (message: TitleQuestion | FeedsQuestion, _sender, respond) => {
          if (message?.cmd === "title") respond(read());
          if (message?.cmd === "feeds") respond(pageFeeds(document));
          return false;
        },
      );
    } catch {
      // the extension was reloaded under this page
    }
  }

  // Every frame says what it shows about a download's checksum: the frame with the link at
  // once, the top page a moment later (its checksum files), so the link's frame answers first.
  try {
    api.runtime.onMessage.addListener((message: ChecksumQuestion, _sender, respond) => {
      if (message?.cmd !== "checksums" || typeof message.url !== "string") return false;
      if (linksTo(document, message.url).length) {
        respond(pageChecksums(document, message.url));
        return false;
      }
      if (window !== window.top) return false;
      setTimeout(() => respond(pageChecksums(document, message.url)), 150);
      return true;
    });
  } catch {
    // the extension was reloaded under this page
  }

  new Overlay().start();
  // Lists whose pages show no video and keep the right-click menu to themselves.
  if (window === window.top && isSpotify(location.href)) new SpotifyButton().start();
  if (window === window.top && isAppleMusic(location.href)) new AppleButton().start();
}
