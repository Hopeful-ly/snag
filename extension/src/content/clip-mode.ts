// The card's Whole video / Clip switch; the trim bar holds the clip.
import { clippable, lengthText, momentText, type Clip } from "../media/clip";
import { button, el, icon, withChildren } from "../shared/dom";
import type { Item } from "../shared/protocol";
import { FILM, SCISSORS } from "./icons";
import type { Trim } from "./trim";

export class ClipMode {
  constructor(
    private trim: Trim,
    private video: () => HTMLVideoElement | null,
    private changed: () => void,
  ) {}

  /** null for the whole video. */
  get clip(): Clip | null {
    return this.trim.clip;
  }

  close(): void {
    this.trim.close();
  }

  render(): HTMLElement[] {
    const video = this.video();
    if (!video || !clippable(video.duration)) return [];
    const clip = this.clip;
    const whole = button(
      "chip" + (clip ? "" : " on"),
      "Download the whole video",
      icon(FILM),
      "Whole video",
    );
    whole.addEventListener("click", () => this.trim.close());
    const part = button(
      "chip" + (clip ? " on" : ""),
      "Download only a part: pick it on the bar over the video",
      icon(SCISSORS),
      "Clip",
    );
    part.addEventListener("click", () => {
      if (this.clip) return;
      this.trim.show(video, this.changed);
      this.changed();
    });
    const parts: HTMLElement[] = [withChildren(el("div", "modes"), whole, part)];
    if (clip) {
      const picked = `${momentText(clip.from)} to ${momentText(clip.to)} (${lengthText(clip)}). `;
      const how =
        "Drag the ends on the bar over the video, then pick what to save: only that part downloads.";
      parts.push(el("p", "clipnote", picked + how));
    }
    return parts;
  }

  size(bytes: number): number {
    const duration = this.video()?.duration || 0;
    const clip = this.clip;
    if (!clip || !clippable(duration)) return bytes;
    return (bytes * (clip.to - clip.from)) / duration;
  }

  apply(item: Item): void {
    const clip = this.clip;
    if (!clip) return;
    item.clip_from = clip.from;
    item.clip_to = clip.to;
    if (item.size) item.size = this.size(item.size);
  }
}
