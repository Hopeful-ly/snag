export interface Clip {
  from: number;
  to: number;
}

/** The shortest clip: shorter than this is a click, not a range. */
const MIN_CLIP = 0.5;
/** A new clip's length, starting where the video is. */
const FIRST_CLIP = 30;

/** Whether a video this long can be clipped (live streams can't: they have no end). */
export function clippable(duration: number): boolean {
  return Number.isFinite(duration) && duration > MIN_CLIP * 2;
}

export function firstClip(at: number, duration: number): Clip {
  const length = Math.min(FIRST_CLIP, duration);
  const from = clamp(at, 0, duration - length);
  return { from: round(from), to: round(from + length) };
}

/** The clip with one end moved to t, kept at least MIN_CLIP long and inside the video. */
export function moveEnd(clip: Clip, end: "from" | "to", t: number, duration: number): Clip {
  if (end === "from") return { from: round(clamp(t, 0, clip.to - MIN_CLIP)), to: clip.to };
  return { from: clip.from, to: round(clamp(t, clip.from + MIN_CLIP, duration)) };
}

export function shift(clip: Clip, dt: number, duration: number): Clip {
  const length = clip.to - clip.from;
  const from = round(clamp(clip.from + dt, 0, duration - length));
  return { from, to: round(from + length) };
}

/** A moment to a tenth of a second: "0:42.0", "1:02:03.5". */
export function momentText(seconds: number): string {
  const tenths = Math.round(Math.max(0, seconds) * 10);
  const s = Math.floor(tenths / 10);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const rest = String(s % 60).padStart(2, "0") + "." + (tenths % 10);
  return h ? `${h}:${String(m).padStart(2, "0")}:${rest}` : `${m}:${rest}`;
}

/** How long a clip is: "36.5 s", "2 min 05 s". */
export function lengthText(clip: Clip): string {
  const length = Math.round((clip.to - clip.from) * 10) / 10;
  if (length < 60) return length.toFixed(1).replace(/\.0$/, "") + " s";
  const s = Math.round(length);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = String(s % 60).padStart(2, "0");
  return h ? `${h} h ${String(m).padStart(2, "0")} min ${sec} s` : `${m} min ${sec} s`;
}

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(v, hi));
const round = (v: number) => Math.round(v * 100) / 100;
