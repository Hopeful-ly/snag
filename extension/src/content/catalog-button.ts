// The button on music sites' list pages, which show no video and keep the right-click menu to
// themselves.
import { alive, ask } from "../shared/ask";
import { api } from "../shared/browser";
import { button, el, icon, pageLayer } from "../shared/dom";
import type { OverlayMessage } from "../shared/protocol";
import { CLOSE, DOWN } from "./icons";
import style from "./overlay.css";

const FOLLOW_EVERY = 1000; // these sites change pages without loading one
const MESSAGE_FOR = 5000;

export class CatalogButton {
  private readonly host: HTMLElement;
  private readonly pill = el("div", "pill page");
  private readonly text: HTMLSpanElement;
  private readonly open: HTMLButtonElement;
  private enabled = true;
  private busy = false;
  private page = "";
  private follow: ReturnType<typeof setInterval> | undefined;
  private messageTimer: ReturnType<typeof setTimeout> | undefined;

  constructor(
    private readonly opts: {
      /** The element left in the page, so an update can take the old one down. */
      host: string;
      listOf: (url: string) => string | null;
      labels: Record<string, string>;
    },
  ) {
    this.host = pageLayer(opts.host);
    this.text = el("span", "", "");
    const root = this.host.attachShadow({ mode: "closed" });
    const sheet = el("style");
    sheet.textContent = style;
    const badge = el("span", "badge");
    badge.append(icon(DOWN));
    this.open = button("open", "Choose what to download in Snag", badge, this.text);
    const hide = button("x", "Hide download button", icon(CLOSE));
    this.pill.append(this.open, hide);
    // Above the site's player bar, clear of its scroll bar.
    this.pill.style.right = "24px";
    this.pill.style.bottom = "104px";
    const wrap = el("div", "wrap");
    wrap.append(this.pill);
    root.append(sheet, wrap);
    for (const type of ["click", "mousedown", "pointerdown", "contextmenu", "keydown"]) {
      wrap.addEventListener(type, (e) => e.stopPropagation()); // the site's own handlers stay out
    }
    this.open.addEventListener("click", () => this.download());
    hide.addEventListener("click", () => {
      this.setEnabled(false);
      ask("overlay", { visible: false }).catch(() => {});
    });
  }

  start(): void {
    document.documentElement.append(this.host);
    this.watch();
    this.update();
    this.follow = setInterval(() => this.update(), FOLLOW_EVERY);
  }

  private update(): void {
    if (!alive()) return this.retire();
    if (location.href !== this.page) {
      this.page = location.href;
      this.showMessage("");
    }
    const list = this.opts.listOf(location.href);
    if (!this.busy && !this.messageTimer)
      this.text.textContent = list ? this.opts.labels[list] || "" : "";
    this.pill.classList.toggle("on", this.enabled && list != null);
  }

  private async download(): Promise<void> {
    if (this.busy) return;
    this.busy = true;
    this.showMessage("Opening in Snag…", false);
    try {
      const found = await ask("playlist", { url: location.href, open: true });
      this.showMessage(found ? "" : "Snag found nothing to download here");
    } catch (error) {
      this.showMessage((error as Error).message || "Snag couldn't open it");
    } finally {
      this.busy = false;
      this.update();
    }
  }

  /** Puts text in the button for a while, or until cleared ("" puts the label back). */
  private showMessage(text: string, expires = true): void {
    clearTimeout(this.messageTimer);
    this.messageTimer = undefined;
    this.open.title = text || "Choose what to download in Snag";
    if (!text) return;
    this.text.textContent = text.length > 80 ? text.slice(0, 79) + "…" : text;
    if (expires) {
      this.messageTimer = setTimeout(() => {
        this.messageTimer = undefined;
        this.update();
      }, MESSAGE_FOR);
    }
  }

  private setEnabled(visible: boolean): void {
    this.enabled = visible;
    this.update();
  }

  /** The toolbar button and X share one hidden state with the Download button on videos. */
  private watch(): void {
    let port: chrome.runtime.Port;
    try {
      port = api.runtime.connect({ name: "overlay" });
    } catch {
      return this.retire();
    }
    port.onMessage.addListener((message: OverlayMessage) => {
      if (message.type === "overlay") this.setEnabled(message.visible);
    });
    port.onDisconnect.addListener(() => {
      if (!alive()) return this.retire();
      setTimeout(() => this.watch(), 200);
    });
  }

  /** After the extension was updated, this copy steps aside for the new one. */
  private retire(): void {
    clearInterval(this.follow);
    this.host.remove();
  }
}
