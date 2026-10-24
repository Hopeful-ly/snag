import type { PlaylistEntry } from "./api/client";
import { duration } from "./format";

// From the entry the page plays (all when none) onward, minus what Snag already has.
export function startingPicks(count: number, current: number, have: boolean[] = []): boolean[] {
  return Array.from({ length: count }, (_, i) => i >= Math.max(current, 0) && !have[i]);
}

export function fromCurrent(count: number, current: number): boolean[] {
  return Array.from({ length: count }, (_, i) => current >= 0 && i >= current);
}

export function toggle(picks: boolean[], i: number, anchor: number | null = null): boolean[] {
  const on = !picks[i];
  const [from, to] = anchor === null ? [i, i] : [Math.min(anchor, i), Math.max(anchor, i)];
  return picks.map((picked, j) => (j >= from && j <= to ? on : picked));
}

export function picked(picks: boolean[]): number[] {
  return picks.flatMap((on, i) => (on ? [i] : []));
}

// "" when no length is known, "over …" when only some are.
export function lengthOf(entries: PlaylistEntry[]): string {
  const known = entries.filter((e) => e.duration);
  if (!known.length) return "";
  const total = duration(known.reduce((sum, e) => sum + (e.duration ?? 0), 0));
  return known.length < entries.length ? `over ${total}` : total;
}

// Some sites list entries without a title: fall back to the last path segment.
export function nameOf(entry: PlaylistEntry): string {
  if (entry.title) return entry.title;
  try {
    const last = new URL(entry.url).pathname.split("/").filter(Boolean).pop();
    return last ? decodeURIComponent(last) : entry.url;
  } catch {
    return entry.url;
  }
}

export function countOf(count: number, audio: boolean): string {
  const noun = audio ? "track" : "video";
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

// "3:07", "1:02:03"; "" when unknown.
export function clockOf(seconds: number | undefined): string {
  if (!seconds) return "";
  const s = Math.round(seconds);
  const two = (n: number) => String(n).padStart(2, "0");
  const [h, m] = [Math.floor(s / 3600), Math.floor((s % 3600) / 60)];
  return h ? `${h}:${two(m)}:${two(s % 60)}` : `${m}:${two(s % 60)}`;
}

// For lengths known to the minute only (TV database runtimes): "~49 min".
export function minutesOf(seconds: number | undefined): string {
  if (!seconds) return "";
  const m = Math.round(seconds / 60);
  return m < 60 ? `~${m} min` : `~${Math.floor(m / 60)} h ${m % 60} min`;
}

interface Group {
  name: string;
  entries: number[];
}

export function groupsOf(entries: PlaylistEntry[]): Group[] {
  const groups = new Map<string, number[]>();
  entries.forEach((e, i) => {
    if (e.group) groups.set(e.group, [...(groups.get(e.group) ?? []), i]);
  });
  return [...groups].map(([name, indexes]) => ({ name, entries: indexes }));
}

export function toggleGroup(picks: boolean[], group: Group): boolean[] {
  const on = !group.entries.every((i) => picks[i]);
  const inGroup = new Set(group.entries);
  return picks.map((picked, i) => (inGroup.has(i) ? on : picked));
}

export function paint(picks: boolean[], from: number, to: number, on: boolean): boolean[] {
  const [low, high] = from < to ? [from, to] : [to, from];
  return picks.map((picked, i) => (i >= low && i <= high ? on : picked));
}
