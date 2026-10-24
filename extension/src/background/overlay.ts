// Hidden state of the Download button (toolbar button or its X); lasts for the browser session.
import { api } from "../shared/browser";
import type { OverlayMessage } from "../shared/protocol";
import { action } from "./badge";
import { paintIcon } from "./icon";

const KEY = "overlayHidden";

let hidden = false;
const pages = new Set<chrome.runtime.Port>();

const ready: Promise<void> = (async () => {
  try {
    const got = await api.storage.session.get(KEY);
    hidden = got[KEY] === true;
  } catch {
    // session storage missing: this background's memory lasts until it goes away
  }
  syncToolbar();
})();

function syncToolbar(): void {
  action
    .setTitle({ title: hidden ? "Show download button" : "Hide download button" })
    ?.catch?.(() => {});
  void paintIcon(!hidden);
}

function tell(): void {
  const message = { type: "overlay", visible: !hidden } satisfies OverlayMessage;
  for (const p of pages) {
    try {
      p.postMessage(message);
    } catch {
      pages.delete(p);
    }
  }
}

export async function overlayVisible(): Promise<boolean> {
  await ready;
  return !hidden;
}

export async function setOverlayVisible(visible: boolean): Promise<boolean> {
  await ready;
  hidden = !visible;
  try {
    await api.storage.session.set({ [KEY]: hidden });
  } catch {
    // memory is enough until this background goes away
  }
  syncToolbar();
  tell();
  return visible;
}

api.runtime.onConnect.addListener((p) => {
  if (p.name !== "overlay") return;
  pages.add(p);
  p.onDisconnect.addListener(() => pages.delete(p));
  void ready.then(() => {
    try {
      p.postMessage({ type: "overlay", visible: !hidden } satisfies OverlayMessage);
    } catch {
      pages.delete(p);
    }
  });
});

action.onClicked.addListener(() => {
  void overlayVisible().then((visible) => setOverlayVisible(!visible));
});
