// Each tab's Spotify web player token and query hashes, so Snag can read a whole playlist, not
// just the 100 songs Spotify shows without them. Kept in memory, only ever sent to local Snag.
import { headersOf, queryOf, SPOTIFY_API } from "../media/spotify";
import { api } from "../shared/browser";
import type { SpotifySession } from "../shared/protocol";

const sessions = new Map<number, SpotifySession>();
const store = api.storage.session;

const ready: Promise<void> = store
  ? store
      .get("spotify")
      .then((saved) => {
        const all = (saved.spotify || {}) as Record<string, SpotifySession>;
        for (const [tab, s] of Object.entries(all)) if (!sessions.has(+tab)) sessions.set(+tab, s);
      })
      .catch(() => {})
  : Promise.resolve();

function save(): void {
  store?.set({ spotify: Object.fromEntries(sessions) }).catch(() => {});
}

function sessionOf(tabId: number): SpotifySession {
  let s = sessions.get(tabId);
  if (!s) {
    s = { token: "", client_token: "", app_version: "", user_agent: "", hashes: {} };
    sessions.set(tabId, s);
  }
  return s;
}

export function watchSpotify(): void {
  api.webRequest.onBeforeRequest.addListener(
    (details) => {
      const query = details.tabId >= 0 && queryOf(details.requestBody?.raw?.[0]?.bytes);
      if (!query) return;
      const s = sessionOf(details.tabId);
      if (s.hashes[query.name] !== query.hash) {
        s.hashes[query.name] = query.hash;
        save();
      }
    },
    { urls: [SPOTIFY_API], types: ["xmlhttprequest"] },
    ["requestBody"],
  );
  api.webRequest.onBeforeSendHeaders.addListener(
    (details) => {
      const seen = details.tabId >= 0 && headersOf(details.requestHeaders);
      if (!seen) return;
      const s = sessionOf(details.tabId);
      if (s.token !== seen.token || s.client_token !== seen.client_token) {
        Object.assign(s, seen);
        save();
      }
    },
    { urls: [SPOTIFY_API], types: ["xmlhttprequest"] },
    ["requestHeaders"],
  );
}

export async function spotifyOf(tabId: number | undefined): Promise<SpotifySession | undefined> {
  await ready;
  const s = tabId != null ? sessions.get(tabId) : undefined;
  return s?.token && Object.keys(s.hashes).length ? s : undefined;
}

export function forgetSpotify(tabId: number): void {
  if (sessions.delete(tabId)) save();
}
