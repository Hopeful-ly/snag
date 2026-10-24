// Lets the page Snag opens to find an episode's stream play in a background tab. Players wait
// for the page to be shown, so page/shown.ts runs in it and in its frames' sites. Where the
// browser can't register main-world scripts (Firefox), the tab opens as it is.
import { api } from "../shared/browser";

const PREFIX = "snag-shown-";
const SCRIPT = "shown.js";

const byTab = new Map<number, Shown>();
let count = 0;

export class Shown {
  private readonly sites = new Set<string>();
  private tabId: number | null = null;
  private pending: Promise<void> = Promise.resolve();

  private constructor(private readonly id: string) {}

  /** null where the browser can't. */
  static async start(page: string): Promise<Shown | null> {
    if (!api.scripting?.registerContentScripts) return null;
    const shown = new Shown(`${PREFIX}${Date.now()}-${count++}`);
    await shown.add(page);
    return shown.sites.size ? shown : null;
  }

  watch(tabId: number): void {
    this.tabId = tabId;
    byTab.set(tabId, this);
  }

  add(url: string): Promise<void> {
    const site = siteOf(url);
    if (!site || this.sites.has(site)) return this.pending;
    this.sites.add(site);
    const first = this.sites.size === 1;
    const script: chrome.scripting.RegisteredContentScript = {
      id: this.id,
      matches: [...this.sites],
      js: [SCRIPT],
      runAt: "document_start",
      allFrames: true,
      world: "MAIN",
      persistAcrossSessions: false,
    };
    // One change at a time: an update must follow the registration it changes.
    this.pending = this.pending
      .then(() =>
        first
          ? api.scripting.registerContentScripts([script])
          : api.scripting.updateContentScripts([script]),
      )
      .catch(() => {
        if (first) this.sites.clear();
      });
    return this.pending;
  }

  async stop(): Promise<void> {
    if (this.tabId != null) byTab.delete(this.tabId);
    await this.pending;
    await api.scripting.unregisterContentScripts({ ids: [this.id] }).catch(() => {});
  }
}

export function frameRequested(tabId: number, url: string): void {
  void byTab.get(tabId)?.add(url);
}

/** Removes registrations an earlier run of the background left, as it stopped mid-way. */
export async function clearShown(): Promise<void> {
  if (!api.scripting?.getRegisteredContentScripts) return;
  const scripts = await api.scripting.getRegisteredContentScripts().catch(() => []);
  const ids = scripts.map((s) => s.id).filter((id) => id.startsWith(PREFIX));
  if (ids.length) await api.scripting.unregisterContentScripts({ ids }).catch(() => {});
}

/** The match pattern for url's site ("https://streamer.example/*"); "" for what isn't a web page. */
export function siteOf(url: string): string {
  try {
    const { protocol, hostname } = new URL(url);
    return protocol === "http:" || protocol === "https:" ? `${protocol}//${hostname}/*` : "";
  } catch {
    return "";
  }
}
