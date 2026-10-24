// Asks Snag to look inside responses that might be a playlist under another name, throttled
// per page so a busy page can't cause a flood.
import { keyOf, type Response } from "../media/detect";
import { api } from "../shared/browser";
import { has, rememberPlaylist } from "./found";
import { send } from "./link";
import { credentials } from "./page";

const PER_PAGE = 40;
const AT_ONCE = 3;

const looked = new Map<number, Set<string>>(); // tab -> addresses already looked at
const queue: { tabId: number; url: string; referer: string }[] = [];
let running = 0;

type Sniffable = Response & { documentUrl?: string; initiator?: string };

/** known: a playlist already listed, read for what it is. */
export function sniff(response: Sniffable, known: boolean): void {
  const key = keyOf(new URL(response.url));
  const done = looked.get(response.tabId) || new Set<string>();
  if (done.has(key) || done.size >= PER_PAGE) return;
  if (!known && has(response.tabId, key)) return;
  done.add(key);
  looked.set(response.tabId, done);
  queue.push({
    tabId: response.tabId,
    url: response.url,
    referer: response.documentUrl || response.initiator || "",
  });
  pump();
}

export function forgetSniffed(tabId: number): void {
  looked.delete(tabId);
}

function pump(): void {
  while (running < AT_ONCE && queue.length) {
    const job = queue.shift()!;
    running++;
    lookInside(job).finally(() => {
      running--;
      pump();
    });
  }
}

async function lookInside(job: { tabId: number; url: string; referer: string }): Promise<void> {
  try {
    const tab = await api.tabs.get(job.tabId).catch(() => null);
    if (!tab) return;
    const reply = await send(
      { type: "sniff", url: job.url, ...(await credentials(job.url, job.referer || tab.url, tab)) },
      30_000,
    );
    if (reply.kind) rememberPlaylist(job.tabId, job.url, reply.kind, reply.text || "");
  } catch {
    // not reachable, or not a playlist
  }
}
