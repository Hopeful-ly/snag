// The trim bar: picks the part of the video to download. It owns the clip; the card reads it.
import { firstClip, lengthText, momentText, moveEnd, shift, type Clip } from "../media/clip";
import { button, el, icon } from "../shared/dom";
import { CLOSE, MARK_IN, MARK_OUT, PAUSE, PLAY } from "./icons";
import { trimPosition, type Box } from "./placement";

type End = "from" | "to";
type Grab = End | "range";

interface Drag {
  grab: Grab;
  pointer: number;
  x: number; // where it was pressed
  start: Clip;
  wasPlaying: boolean;
}

const SEEK_EVERY = 80; // ms between seeks while dragging, so the player keeps up
const NUDGE = 1; // seconds an arrow key moves an end; with Shift, a tenth of that

export class Trim {
  readonly el = el("div", "trim");
  /** null while the bar is closed. */
  clip: Clip | null = null;

  private video: HTMLVideoElement | null = null;
  private onChange: () => void = () => {};
  private drag: Drag | null = null;
  private playingClip = false;
  private lastSeek = 0;

  private readonly times = el("span", "trim-times mono");
  private readonly length = el("span", "trim-len");
  private readonly rail = el("div", "rail");
  private readonly sel = el("div", "sel");
  private readonly head = el("div", "playhead");
  private readonly handles = {
    from: button("handle from", "Start of the clip · ← → nudge, Shift for a tenth"),
    to: button("handle to", "End of the clip · ← → nudge, Shift for a tenth"),
  };
  private readonly play = button("tbtn", "Play the clip", icon(PLAY));

  constructor() {
    const markIn = button("tbtn", "Start the clip where the video is", icon(MARK_IN));
    const markOut = button("tbtn", "End the clip where the video is", icon(MARK_OUT));
    const close = button("tbtn", "Back to the whole video", icon(CLOSE));
    const top = el("div", "trim-top");
    top.append(this.times, this.length, el("span", "grow"), markIn, this.play, markOut, close);
    this.rail.append(el("div", "track"), this.sel, this.head, this.handles.from, this.handles.to);
    this.el.append(top, this.rail);

    markIn.addEventListener("click", () => this.mark("from"));
    markOut.addEventListener("click", () => this.mark("to"));
    this.play.addEventListener("click", () => this.togglePlay());
    close.addEventListener("click", () => this.close());
    this.rail.addEventListener("pointerdown", (e) => this.press(e));
    this.rail.addEventListener("pointermove", (e) => this.move(e));
    this.rail.addEventListener("pointerup", (e) => this.release(e));
    this.rail.addEventListener("pointercancel", (e) => this.release(e));
    for (const end of ["from", "to"] as const) {
      this.handles[end].addEventListener("keydown", (e) => this.nudge(e, end));
    }
  }

  get open(): boolean {
    return this.clip !== null;
  }

  /** onChange fires once per drag, not on every move, and when the bar closes. */
  show(video: HTMLVideoElement, onChange: () => void): void {
    this.close();
    this.video = video;
    this.onChange = onChange;
    this.clip = firstClip(video.currentTime || 0, this.duration());
    video.addEventListener("timeupdate", this.onTime);
    this.el.classList.add("on");
    this.paint();
  }

  close(): void {
    if (!this.clip) return;
    this.stopPlaying();
    this.video?.removeEventListener("timeupdate", this.onTime);
    this.clip = null;
    this.drag = null;
    this.el.classList.remove("on");
    this.onChange();
  }

  /** Called every frame while the bar shows. */
  place(video: Box, card: Box | null): void {
    if (!this.clip) return;
    const view = { width: innerWidth, height: innerHeight };
    const at = trimPosition(video, card, this.el.offsetHeight || 74, view);
    this.el.style.left = at.left + "px";
    this.el.style.top = at.top + "px";
    this.el.style.width = at.width + "px";
    this.paintHead();
  }

  private duration(): number {
    const d = this.video?.duration || 0;
    return Number.isFinite(d) ? d : 0;
  }

  /** tell is false while dragging, so the card hears of it once, at the end. */
  private set(clip: Clip, tell = true): void {
    this.clip = clip;
    this.paint();
    if (tell) this.onChange();
  }

  private paint(): void {
    const clip = this.clip;
    const d = this.duration();
    if (!clip || !d) return;
    this.times.textContent = momentText(clip.from) + " – " + momentText(clip.to);
    this.length.textContent = lengthText(clip);
    const from = (clip.from / d) * 100;
    const to = (clip.to / d) * 100;
    this.sel.style.left = from + "%";
    this.sel.style.width = to - from + "%";
    this.handles.from.style.left = from + "%";
    this.handles.to.style.left = to + "%";
    this.paintHead();
  }

  private paintHead(): void {
    const d = this.duration();
    if (this.video && d) this.head.style.left = (this.video.currentTime / d) * 100 + "%";
  }

  private readonly onTime = () => {
    this.paintHead();
    if (this.playingClip && this.clip && this.video && this.video.currentTime >= this.clip.to) {
      this.video.pause();
      this.stopPlaying();
    }
  };

  private mark(end: End): void {
    if (this.clip && this.video) {
      this.set(moveEnd(this.clip, end, this.video.currentTime, this.duration()));
    }
  }

  private nudge(e: KeyboardEvent, end: End): void {
    if (!this.clip || (e.key !== "ArrowLeft" && e.key !== "ArrowRight")) return;
    e.preventDefault();
    const step = (e.shiftKey ? NUDGE / 10 : NUDGE) * (e.key === "ArrowLeft" ? -1 : 1);
    this.set(moveEnd(this.clip, end, this.clip[end] + step, this.duration()));
    this.seek(this.clip[end]);
  }

  private timeAt(x: number): number {
    const r = this.rail.getBoundingClientRect();
    return Math.max(0, Math.min((x - r.left) / r.width, 1)) * this.duration();
  }

  private press(e: PointerEvent): void {
    if (e.button !== 0 || !this.clip || !this.video) return;
    e.preventDefault();
    const grab = grabbed(e.target as Element);
    if (!grab) {
      this.seek(this.timeAt(e.clientX)); // the rest of the rail is a timeline
      return;
    }
    this.stopPlaying();
    this.drag = {
      grab,
      pointer: e.pointerId,
      x: e.clientX,
      start: { ...this.clip },
      wasPlaying: !this.video.paused,
    };
    this.video.pause(); // the frame under the handle shows while dragging
    try {
      this.rail.setPointerCapture(e.pointerId);
    } catch {
      // the pointer is already gone
    }
    this.el.classList.add("dragging");
    if (grab !== "range") this.handles[grab].focus();
  }

  private move(e: PointerEvent): void {
    const drag = this.drag;
    if (!drag || e.pointerId !== drag.pointer) return;
    const d = this.duration();
    const clip =
      drag.grab === "range"
        ? shift(drag.start, this.timeAt(e.clientX) - this.timeAt(drag.x), d)
        : moveEnd(drag.start, drag.grab, this.timeAt(e.clientX), d);
    this.set(clip, false);
    this.seek(drag.grab === "to" ? clip.to : clip.from);
  }

  private release(e: PointerEvent): void {
    const drag = this.drag;
    if (!drag || e.pointerId !== drag.pointer) return;
    this.drag = null;
    this.el.classList.remove("dragging");
    if (!this.clip) return;
    this.seek(drag.grab === "to" ? this.clip.to : this.clip.from); // where it was let go
    this.onChange();
    if (drag.wasPlaying) this.video?.play().catch(() => {});
  }

  /** While dragging, at most every SEEK_EVERY ms. */
  private seek(t: number): void {
    const video = this.video;
    if (!video) return;
    const now = performance.now();
    if (this.drag && now - this.lastSeek < SEEK_EVERY) return;
    this.lastSeek = now;
    if (this.drag && typeof video.fastSeek === "function") video.fastSeek(t);
    else video.currentTime = t;
    this.paintHead();
  }

  private togglePlay(): void {
    const video = this.video;
    if (!video || !this.clip) return;
    if (this.playingClip) {
      video.pause();
      this.stopPlaying();
      return;
    }
    const at = video.currentTime;
    if (at < this.clip.from || at >= this.clip.to - 0.2) video.currentTime = this.clip.from;
    this.playingClip = true;
    this.play.replaceChildren(icon(PAUSE));
    this.play.title = "Pause";
    video.play().catch(() => this.stopPlaying());
  }

  private stopPlaying(): void {
    if (!this.playingClip) return;
    this.playingClip = false;
    this.play.replaceChildren(icon(PLAY));
    this.play.title = "Play the clip";
  }
}

/** What a press on the rail took hold of: an end, the whole range, or nothing (the timeline). */
function grabbed(target: Element): Grab | null {
  if (target.closest(".handle.from")) return "from";
  if (target.closest(".handle.to")) return "to";
  if (target.closest(".sel")) return "range";
  return null;
}
