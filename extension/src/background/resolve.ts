// Finds the stream a streaming site's page plays, when Snag asks: the page plays for a moment in a
// muted background tab made to look shown (shown.ts). Stream links expire, so Snag asks per episode.
import { pickStream } from "../media/resolve";
import { api } from "../shared/browser";
import { mediaOf } from "./found";
import { send } from "./link";
import { credentials } from "./page";
import { Shown } from "./shown";

// How long a page gets to start playing, and how long to wait once a stream showed up, in case
// a better one (all the qualities, past an ad) follows.
const GIVE_UP = 45_000;
const SETTLE = 2_500;
const POLL = 400;

export async function resolvePage(token: string, page: string): Promise<void> {
  let tabId: number | undefined;
  const shown = await Shown.start(page);
  try {
    const tab = await api.tabs.create({ url: page, active: false });
    tabId = tab.id;
    if (tabId == null) throw new Error("the browser opened no tab");
    shown?.watch(tabId);
    await api.tabs.update(tabId, { muted: true }).catch(() => {});
    const stream = await waitForStream(tabId);
    const extra = await credentials(stream.url, stream.referer || page, tab);
    await send({ type: "resolved", token, url: stream.url, ...extra });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    await send({ type: "resolve_failed", token, error: message }).catch(() => {});
  } finally {
    if (tabId != null) await api.tabs.remove(tabId).catch(() => {});
    await shown?.stop();
  }
}

async function waitForStream(tabId: number) {
  const start = Date.now();
  let firstSeen = 0;
  for (;;) {
    const stream = pickStream(mediaOf(tabId));
    if (stream) {
      firstSeen ||= Date.now();
      if (
        stream.media === "dash" ||
        stream.info?.type === "master" ||
        Date.now() - firstSeen >= SETTLE
      )
        return stream;
    }
    if (Date.now() - start > GIVE_UP) throw new Error("no video started playing on the page");
    await new Promise((resolve) => setTimeout(resolve, POLL));
  }
}
