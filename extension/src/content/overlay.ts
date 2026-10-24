// The Download button on videos. Button and card can be dragged; each site remembers where.
import { alive, ask } from "../shared/ask";
import { api } from "../shared/browser";
import type { OverlayMessage } from "../shared/protocol";
import { Card } from "./card";
import { button, el, icon, pageLayer } from "../shared/dom";
import { CLOSE, DOWN, GRIP, MINIMIZE } from "./icons";
import style from "./overlay.css";
import { Trim } from "./trim";
import {
  CARD_WIDTH,
  DEFAULT_SPOT,
  cardPosition,
  dragged,
  isSpot,
  pillPosition,
  roomFor,
  snapped,
  type Spot,
} from "./placement";

const MIN_WIDTH = 200; // smaller videos are previews and thumbnails
const MIN_HEIGHT = 112;
const HIDE_DELAY = 1400;
const SHOW_ON_PLAY = 3500;
const DOUBLE_CLICK = 400;
const HANDLE = "Drag to move · double-click to put back";

// The extension's storage, like its messages, is gone once the extension was reloaded.
function storage<T>(use: (s: chrome.storage.StorageArea) => Promise<T>): Promise<T | null> {
  try {
    return alive() ? use(api.storage.local).catch(() => null) : Promise.resolve(null);
  } catch {
    return Promise.resolve(null);
  }
}

interface Drag {
  x: number;
  y: number;
  from: Spot;
  moved: boolean;
  el: HTMLElement;
  pointer: number;
  onOpen: boolean; // pressed on "Download"
  onHandle: boolean; // pressed on the grip or the card's header
}

export class Overlay {
  private readonly host = pageLayer("snag-overlay");
  private readonly pill: HTMLElement;
  private readonly cardEl: HTMLElement;
  private readonly card: Card;
  private readonly trim = new Trim();
  private readonly positionKey = "snagpos:" + location.hostname;

  private enabled = true;
  private retired = false;
  private video: HTMLVideoElement | null = null;
  private visible = false;
  private hideTimer: ReturnType<typeof setTimeout> | undefined;
  private frame = 0;
  private pointer = { x: -1, y: -1 };
  private spot: Spot = { ...DEFAULT_SPOT };
  private drag: Drag | null = null;
  private lastHandleClick = 0;

  constructor() {
    const root = this.host.attachShadow({ mode: "closed" });
    const sheet = el("style");
    sheet.textContent = style;

    const grip = el("div", "grip");
    grip.title = HANDLE;
    grip.append(icon(GRIP));
    const badge = () => {
      const b = el("span", "badge");
      b.append(icon(DOWN));
      return b;
    };
    const openButton = button("open", "Download with Snag", badge(), el("span", "", "Download"));
    const hideButton = button("x", "Hide download button", icon(CLOSE));
    this.pill = el("div", "pill");
    this.pill.append(grip, openButton, hideButton);

    const title = el("div", "title");
    const subtitle = el("div", "sub");
    const titles = el("div", "titles");
    titles.append(title, subtitle);
    const minimize = button("hbtn min", "Minimize", icon(MINIMIZE));
    const shut = button("hbtn shut", "Hide download button", icon(CLOSE));
    const head = el("div", "head");
    head.title = HANDLE;
    head.append(badge(), titles, minimize, shut);
    const body = el("div", "body");
    const foot = el("div", "foot");
    this.cardEl = el("div", "card");
    this.cardEl.setAttribute("role", "dialog");
    this.cardEl.setAttribute("aria-label", "Download with Snag");
    this.cardEl.style.width = CARD_WIDTH + "px";
    this.cardEl.append(head, body, foot);

    const wrap = el("div", "wrap");
    wrap.append(this.pill, this.cardEl, this.trim.el);
    root.append(sheet, wrap);

    this.card = new Card({
      title,
      subtitle,
      body,
      foot,
      video: () => this.video,
      trim: this.trim,
      done: () => {
        this.closeCard();
        this.scheduleHide(1200);
      },
      reopen: () => this.openCard(),
    });

    // Keep the page's player from reacting to clicks on the button.
    for (const type of [
      "click",
      "mousedown",
      "mouseup",
      "pointerdown",
      "pointerup",
      "dblclick",
      "contextmenu",
      "wheel",
      "keydown",
    ]) {
      wrap.addEventListener(type, (e) => e.stopPropagation());
    }
    // A press and release on "Download" opens the card (see endDrag); this is for the keyboard.
    openButton.addEventListener("click", (e) => e.detail === 0 && this.toggle());
    for (const b of [hideButton, shut]) {
      b.addEventListener("click", () => this.hideForSession());
    }
    minimize.addEventListener("click", () => this.closeCard());
    for (const handle of [this.pill, head]) {
      handle.addEventListener("pointerdown", (e) => this.startDrag(e));
      handle.addEventListener("pointermove", (e) => this.moveDrag(e));
      handle.addEventListener("pointerup", (e) => this.endDrag(e));
      handle.addEventListener("pointercancel", (e) => this.endDrag(e));
    }
  }

  start(): void {
    this.watch();
    ask("overlay")
      .then((s) => this.applyVisible(s.visible))
      .catch(() => {});
    storage((s) => s.get(this.positionKey)).then((saved) => {
      const spot = saved?.[this.positionKey];
      if (isSpot(spot)) this.spot = spot;
    });

    addEventListener("pointermove", this.onPointerMove, { capture: true, passive: true });
    document.addEventListener(
      "play",
      (e) => {
        const video = e.target;
        if (!(video instanceof HTMLVideoElement) || !this.enabled || !big(video)) return;
        if (video.muted && video.loop) return; // background and preview clips
        this.show(video);
        this.scheduleHide(SHOW_ON_PLAY);
      },
      true,
    );
    document.addEventListener("fullscreenchange", () => {
      if (!this.visible) return;
      this.attach();
      this.place();
    });
  }

  /** The toolbar button and X share one hidden state for this browser session. */
  private watch(): void {
    if (this.retired || !alive()) return this.retire();
    let port: chrome.runtime.Port;
    try {
      port = api.runtime.connect({ name: "overlay" });
    } catch {
      return this.retire();
    }
    port.onMessage.addListener((message: OverlayMessage) => {
      if (message.type === "overlay") this.applyVisible(message.visible);
    });
    port.onDisconnect.addListener(() => {
      if (this.retired || !alive()) return this.retire();
      setTimeout(() => this.watch(), 200);
    });
  }

  private applyVisible(visible: boolean): void {
    this.enabled = visible;
    if (!visible) this.hide(true);
    else this.check();
  }

  private hideForSession(): void {
    this.applyVisible(false);
    ask("overlay", { visible: false }).catch(() => {});
  }

  private readonly onPointerMove = (e: PointerEvent) => {
    if (!alive()) return this.retire();
    this.pointer = { x: e.clientX, y: e.clientY };
    if (!this.frame) this.frame = requestAnimationFrame(() => this.check());
  };

  /** After the extension was updated, this copy steps aside for the new one. */
  private retire(): void {
    this.retired = true;
    removeEventListener("pointermove", this.onPointerMove, true);
    this.host.remove();
  }

  private check(): void {
    this.frame = 0;
    if (!this.enabled) return;
    const { x, y } = this.pointer;
    if (this.drag || (this.visible && this.overUs(x, y))) {
      clearTimeout(this.hideTimer);
      this.hideTimer = undefined;
      return;
    }
    const video = videoAt(x, y);
    if (video) {
      this.show(video);
      clearTimeout(this.hideTimer);
      this.hideTimer = undefined;
    } else if (this.visible && !this.card.open && !this.hideTimer) {
      this.scheduleHide(HIDE_DELAY);
    }
  }

  private overUs(x: number, y: number): boolean {
    const shown = [this.pill];
    if (this.card.open) shown.push(this.cardEl);
    if (this.trim.open) shown.push(this.trim.el);
    for (const e of shown) {
      const r = e.getBoundingClientRect();
      if (x >= r.left - 6 && x <= r.right + 6 && y >= r.top - 6 && y <= r.bottom + 6) return true;
    }
    return false;
  }

  private scheduleHide(delay: number): void {
    clearTimeout(this.hideTimer);
    this.hideTimer = setTimeout(() => {
      this.hideTimer = undefined;
      if (!this.card.open && !this.drag && !this.overUs(this.pointer.x, this.pointer.y))
        this.hide();
    }, delay);
  }

  private attach(): void {
    const full = document.fullscreenElement;
    const parent = full && full.tagName !== "VIDEO" ? full : document.body;
    if (!parent) return;
    if (this.host.parentNode !== parent) parent.appendChild(this.host);
  }

  private show(video: HTMLVideoElement): void {
    if (video !== this.video && this.card.open) this.closeCard();
    this.video = video;
    this.attach();
    this.card.prepare();
    if (this.visible) return;
    this.visible = true;
    this.place();
    requestAnimationFrame(() => this.pill.classList.add("on"));
    this.follow();
  }

  private hide(now = false): void {
    this.visible = false;
    this.pill.classList.remove("on");
    this.closeCard();
    if (now) this.video = null;
  }

  private follow(): void {
    if (!this.visible) return;
    const v = this.video;
    if (!v || !v.isConnected || document.fullscreenElement === v || !big(v)) {
      this.hide(true);
      return;
    }
    this.attach();
    this.place();
    requestAnimationFrame(() => this.follow());
  }

  private pillSize() {
    return { width: this.pill.offsetWidth || 150, height: this.pill.offsetHeight || 38 };
  }

  private place(): void {
    if (!this.video) return;
    const video = this.video.getBoundingClientRect();
    const view = { width: innerWidth, height: innerHeight };
    const at = pillPosition(video, this.pillSize(), this.spot, view);
    this.pill.style.left = at.left + "px";
    this.pill.style.top = at.top + "px";
    if (!this.card.open) return;
    const height = this.cardEl.offsetHeight || 300;
    const card = cardPosition(video, this.pillSize(), at, height, view);
    this.cardEl.style.left = card.left + "px";
    this.cardEl.style.top = card.top + "px";
    this.trim.place(video, { ...card, width: CARD_WIDTH, height });
  }

  private toggle(): void {
    if (this.card.open) this.closeCard();
    else this.openCard();
  }

  private openCard(): void {
    this.pill.style.visibility = "hidden"; // the card takes its place
    this.cardEl.classList.add("on");
    clearTimeout(this.hideTimer);
    this.card.show();
    this.place();
  }

  private closeCard(): void {
    this.card.hide();
    this.pill.style.visibility = "";
    this.cardEl.classList.remove("on");
  }

  // The handle takes the pointer as soon as it is pressed: the pointer leaves the small button at
  // once, and the page would get the moves and the release instead.
  private startDrag(e: PointerEvent): void {
    const target = e.target as Element;
    if (e.button !== 0 || target.closest?.("button.x, button.hbtn")) return;
    this.drag = {
      x: e.clientX,
      y: e.clientY,
      from: { ...this.spot },
      moved: false,
      el: e.currentTarget as HTMLElement,
      pointer: e.pointerId,
      onOpen: !!target.closest?.("button.open"),
      onHandle:
        !!target.closest?.(".grip") || (!!target.closest?.(".head") && !target.closest?.("button")),
    };
    try {
      this.drag.el.setPointerCapture(e.pointerId);
    } catch {
      // the pointer is already gone
    }
    e.preventDefault();
  }

  private moveDrag(e: PointerEvent): void {
    const drag = this.drag;
    if (!drag || e.pointerId !== drag.pointer || !this.video) return;
    const dx = e.clientX - drag.x;
    const dy = e.clientY - drag.y;
    if (!drag.moved) {
      if (Math.hypot(dx, dy) < 4) return;
      drag.moved = true;
      this.pill.classList.add("dragging");
      this.cardEl.classList.add("dragging");
    }
    this.spot = dragged(
      drag.from,
      dx,
      dy,
      roomFor(this.video.getBoundingClientRect(), this.pillSize()),
    );
    this.place();
  }

  private endDrag(e: PointerEvent): void {
    const drag = this.drag;
    if (!drag || e.pointerId !== drag.pointer) return;
    this.drag = null;
    if (drag.moved) {
      this.spot = snapped(this.spot);
      this.pill.classList.remove("dragging");
      this.cardEl.classList.remove("dragging");
      this.place();
      storage((s) => s.set({ [this.positionKey]: this.spot }));
      return;
    }
    if (e.type !== "pointerup") return;
    if (drag.onOpen) this.toggle();
    // Two quick clicks on the grip or the card's header put it back. (The pointer is captured,
    // so the browser's own dblclick lands elsewhere.)
    if (drag.onHandle) {
      if (Date.now() - this.lastHandleClick < DOUBLE_CLICK) {
        this.lastHandleClick = 0;
        this.spot = { ...DEFAULT_SPOT };
        this.place();
        storage((s) => s.remove(this.positionKey));
      } else {
        this.lastHandleClick = Date.now();
      }
    }
  }
}

function big(video: HTMLVideoElement): boolean {
  const r = video.getBoundingClientRect();
  return (
    r.width >= MIN_WIDTH &&
    r.height >= MIN_HEIGHT &&
    r.bottom > 0 &&
    r.right > 0 &&
    r.top < innerHeight &&
    r.left < innerWidth &&
    getComputedStyle(video).visibility !== "hidden"
  );
}

function videoAt(x: number, y: number): HTMLVideoElement | null {
  for (const video of document.getElementsByTagName("video")) {
    const r = video.getBoundingClientRect();
    if (x >= r.left && x <= r.right && y >= r.top && y <= r.bottom && big(video)) return video;
  }
  return null;
}
