// What the page offers the card, kept up to date while it is open.
import { tracksOf } from "../media/captions";
import { captionForPage, meaningfulName } from "../media/detect";
import {
  captionsOf,
  markNeedsPlayer,
  options,
  probeStreams,
  type Caption,
  type Option,
  type StreamProbes,
} from "../media/offers";
import { ask } from "../shared/ask";
import { isWebAddress } from "../shared/format";
import type { Media, ProbeResult, SeenEntry } from "../shared/protocol";

const REFRESH_EVERY = 2000;

interface Probe {
  loading: boolean;
  result: ProbeResult | null;
  error: string;
}

export class PageOffers {
  probe: Probe = { loading: true, result: null, error: "" };
  pageTitle = "";

  private media: Media[] = [];
  private fetched: Media[] = []; // caption files the page loaded
  private streams: StreamProbes = {};
  private refresh: ReturnType<typeof setInterval> | undefined;
  private lastCaptions = "";
  private unwatch: (() => void) | undefined;
  private prepared = ""; // the address the page was last probed at

  constructor(
    private video: () => HTMLVideoElement | null,
    private changed: () => void,
  ) {}

  /** Asking Snag takes seconds; the background shares the answer when the card asks again. */
  prepare(): void {
    if (this.prepared === location.href) return;
    this.prepared = location.href; // pages that change address without loading (YouTube) ask again
    ask("probe", { frameUrl: location.href }).catch(() => {});
  }

  start(): void {
    this.probe = { loading: true, result: null, error: "" };
    this.fetched = [];
    this.lastCaptions = "";
    ask("pageTitle")
      .then((title) => {
        this.pageTitle = title || "";
        this.changed();
      })
      .catch(() => {});
    this.load();
    this.watchTracks();
    clearInterval(this.refresh);
    this.refresh = setInterval(() => this.load(), REFRESH_EVERY);
    ask("probe", { frameUrl: location.href })
      .then(
        (result) => (this.probe = { loading: false, result, error: "" }),
        (error: Error) => (this.probe = { loading: false, result: null, error: error.message }),
      )
      .then(() => this.changed());
  }

  stop(): void {
    this.unwatch?.();
    this.unwatch = undefined;
    clearInterval(this.refresh);
  }

  options(): { video: Option[]; audio: Option[]; files: Option[] } {
    return options(this.probe.result, this.mediaWithDirect(), this.streams);
  }

  loading(): boolean {
    return this.probe.loading || Object.values(this.streams).some((p) => p.loading);
  }

  title(): string {
    const probe = this.probe.result;
    return (probe?.supported && probe.title) || this.pageTitle || document.title || "This video";
  }

  /** Seconds; 0 when unknown. */
  duration(): number {
    return (
      this.probe.result?.duration || Math.max(0, ...this.media.map((m) => m.info?.duration || 0))
    );
  }

  captions(): Caption[] {
    let captions: Caption[];
    try {
      captions = captionsOf(this.probe.result, this.streams, this.fetched, tracksOf(this.video()));
    } catch {
      captions = captionsOf(this.probe.result, this.streams, this.fetched);
    }
    return markNeedsPlayer(captions, location.href);
  }

  private load(): void {
    ask("seen", { entries: seenEntries() })
      .then((page) => {
        const sizes = (l: Media[]) => JSON.stringify(l.map((m) => m.key + m.size));
        const mediaChanged = sizes(page.media) !== sizes(this.media);
        if (mediaChanged) this.media = page.media;
        this.fetched = page.captions.filter((m) => captionForPage(m.url, location.href));
        let captions: string;
        try {
          captions = this.captionKey();
        } catch {
          captions = this.lastCaptions;
        }
        if (mediaChanged || captions !== this.lastCaptions) {
          this.lastCaptions = captions;
          this.changed();
        }
        probeStreams(
          this.media,
          this.streams,
          (url) => ask("probeStream", { url }),
          () => this.changed(),
        );
      })
      .catch(() => {});
  }

  private captionKey(): string {
    return this.captions()
      .map((c) => c.code + ":" + (c.url || "") + ":" + (c.text?.length || 0))
      .join("|");
  }

  private watchTracks(): void {
    this.unwatch?.();
    this.unwatch = undefined;
    const tracks = this.video()?.textTracks;
    if (!tracks) return;
    const on = () => this.changed();
    try {
      tracks.addEventListener("addtrack", on);
      tracks.addEventListener("change", on);
    } catch {
      return;
    }
    this.unwatch = () => {
      tracks.removeEventListener("addtrack", on);
      tracks.removeEventListener("change", on);
    };
  }

  private mediaWithDirect(): Media[] {
    const list = [...this.media];
    const video = this.video();
    const direct = video?.currentSrc || video?.src || "";
    if (isWebAddress(direct) && !list.some((m) => m.url === direct)) {
      list.push({
        key: direct,
        url: direct,
        media: "video",
        kind: "http",
        mime: "",
        size: 0,
        time: 0,
        frame: 0,
        // The browser may not have reported loading it (it played before Snag was watching, or
        // never loaded), so the address is all there is to name it by.
        filename: meaningfulName(direct) || undefined,
      });
    }
    return list.reverse().slice(0, 16);
  }
}

/** The page's own resource list (may predate the extension watching this tab). */
function seenEntries(): SeenEntry[] {
  try {
    return (performance.getEntriesByType("resource") as PerformanceResourceTiming[])
      .filter((e) => /\.(m3u8|mpd|mp4|m4v|webm|mkv|mov|mp3|m4a|ogg|opus)(\?|$)/i.test(e.name))
      .map((e) => ({ url: e.name, initiator: e.initiatorType }));
  } catch {
    return [];
  }
}
