// The card's subtitle choice: off or a language, embedded into the file or as a .srt beside it.
import type { Caption } from "../media/offers";
import { button, el, withChildren } from "../shared/dom";
import type { Item } from "../shared/protocol";

export class SubtitlePicker {
  private code = ""; // the language picked; "" for none
  private sidecar = false;

  constructor(private changed: () => void) {}

  reset(): void {
    this.code = "";
    this.sidecar = false;
  }

  render(captions: Caption[]): HTMLElement[] {
    if (!captions.length) return [];
    const parts = [el("div", "label", "Subtitles"), this.languages(captions)];
    if (this.code) parts.push(this.how(), this.note(captions));
    return parts;
  }

  /** A download of the sound alone gets none. */
  apply(item: Item, captions: Caption[]): void {
    const sub = captions.find((s) => s.code === this.code);
    if (!sub || item.audio) return;
    item.subs = sub.code;
    if (sub.auto) item.sub_auto = true;
    if (this.sidecar) item.sidecar = true;
    if (sub.url) item.sub_url = sub.url;
    if (sub.text) item.sub_text = sub.text;
  }

  private languages(captions: Caption[]): HTMLElement {
    const off = this.chip(!this.code, "No captions", "Off", () => (this.code = ""));
    const langs = captions.map((s) =>
      this.chip(this.code === s.code, s.auto ? s.name + " (automatic)" : s.name, s.name, () => {
        this.code = s.code;
      }),
    );
    return withChildren(el("div", "chips langs"), off, ...langs);
  }

  private how(): HTMLElement {
    return withChildren(
      el("div", "chips langs"),
      this.chip(!this.sidecar, "Embed captions into the video file", "Embed into file", () => {
        this.sidecar = false;
      }),
      this.chip(
        this.sidecar,
        "Save captions as a .srt next to the video",
        "Separate .srt file",
        () => {
          this.sidecar = true;
        },
      ),
    );
  }

  private chip(on: boolean, title: string, text: string, pick: () => void): HTMLButtonElement {
    const b = button("chip" + (on ? " on" : ""), title, text);
    b.addEventListener("click", () => {
      pick();
      this.changed();
    });
    return b;
  }

  private note(captions: Caption[]): HTMLElement {
    const sub = captions.find((s) => s.code === this.code);
    const name = sub?.name || "These";
    if (sub?.needsPlayer) {
      // They show up here with their file once the player loads them, and this note goes.
      return el(
        "p",
        "note warn",
        `YouTube gives captions to its own player alone: turn on ${name} in the player (CC) ` +
          "first, or the video is saved without them.",
      );
    }
    if (this.sidecar) return el("p", "note", name + " as a separate .srt next to the video.");
    return el("p", "note", name + " embedded in the file. Most players let you turn them on.");
  }
}
