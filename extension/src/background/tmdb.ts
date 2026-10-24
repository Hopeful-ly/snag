// The TMDB API mirror each tab's page read its show from, so Snag can list the episodes.
import { tmdbCopyOf } from "../media/resolve";
import { api } from "../shared/browser";

const copies = new Map<number, string>();
const session = api.storage.session;

const ready: Promise<void> = session
  ? session
      .get("tmdb")
      .then((saved) => {
        for (const [tab, base] of Object.entries((saved.tmdb || {}) as Record<string, string>))
          if (!copies.has(+tab)) copies.set(+tab, base);
      })
      .catch(() => {})
  : Promise.resolve();

function save(): void {
  session?.set({ tmdb: Object.fromEntries(copies) }).catch(() => {});
}

export function noteRequest(tabId: number, url: string): void {
  if (tabId < 0) return;
  const base = tmdbCopyOf(url);
  const known = copies.get(tabId);
  // A request without the page's key doesn't make one that had it forgotten.
  if (known?.startsWith(base + "?api_key=")) return;
  if (base && known !== base) {
    copies.set(tabId, base);
    save();
  }
}

export async function tmdbOf(tabId: number | undefined): Promise<string> {
  await ready;
  return (tabId != null && copies.get(tabId)) || "";
}

export function forgetTMDB(tabId: number): void {
  if (copies.delete(tabId)) save();
}
