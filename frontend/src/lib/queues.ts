import type { Countdown, Download, Queue, QueueState } from "./api/client";
import { duration } from "./format";

export const mainQueue = "main";

// Main stays first: dropped on Main, a queue goes to the top of the rest.
export function arranged(ids: string[], moving: string, target: string, after: boolean): string[] {
  if (moving === mainQueue || moving === target || !ids.includes(moving)) return ids;
  const rest = ids.filter((id) => id !== moving);
  let at = target === mainQueue ? 1 : rest.indexOf(target) + (after ? 1 : 0);
  if (at < 1) at = rest.length;
  return [...rest.slice(0, at), moving, ...rest.slice(at)];
}

// Schedule numbering (0 is Sunday), in Monday-first order.
export const weekdays = [
  { day: 1, short: "Mon" },
  { day: 2, short: "Tue" },
  { day: 3, short: "Wed" },
  { day: 4, short: "Thu" },
  { day: 5, short: "Fri" },
  { day: 6, short: "Sat" },
  { day: 0, short: "Sun" },
];

export const whenDoneChoices = [
  { id: "", label: "Nothing" },
  { id: "sleep", label: "Sleep" },
  { id: "shutdown", label: "Shut down" },
  { id: "quit", label: "Quit Snag" },
];

// null when it isn't a time.
export function minutesOf(hhmm: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  if (!m) return null;
  const h = Number(m[1]);
  const min = Number(m[2]);
  return h < 24 && min < 60 ? h * 60 + min : null;
}

export function clock(hhmm: string): string {
  const minutes = minutesOf(hhmm);
  if (minutes === null) return hhmm;
  const t = new Date(2000, 0, 1, Math.floor(minutes / 60), minutes % 60);
  return t.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

export function daysText(days: number[] | undefined): string {
  const set = new Set(days ?? []);
  if (set.size === 0 || set.size === 7) return "every day";
  if (set.size === 5 && [1, 2, 3, 4, 5].every((d) => set.has(d))) return "on weekdays";
  if (set.size === 2 && set.has(0) && set.has(6)) return "on weekends";
  return (
    "on " +
    weekdays
      .filter((w) => set.has(w.day))
      .map((w) => w.short)
      .join(", ")
  );
}

export function scheduleText(q: Queue): string {
  const s = q.schedule;
  if (!s?.on || minutesOf(s.start) === null) return "";
  const hours = s.stop ? `${clock(s.start)} to ${clock(s.stop)}` : `Starts at ${clock(s.start)}`;
  return `${hours}, ${daysText(s.days)}`;
}

export function nextStart(q: Queue, now = new Date()): Date | null {
  const s = q.schedule;
  const minutes = s?.on ? minutesOf(s.start) : null;
  if (minutes === null) return null;
  const days = new Set(s.days ?? []);
  for (let ahead = 0; ahead <= 7; ahead++) {
    const t = new Date(now.getFullYear(), now.getMonth(), now.getDate() + ahead);
    t.setHours(Math.floor(minutes / 60), minutes % 60, 0, 0);
    if (t > now && (days.size === 0 || days.has(t.getDay()))) return t;
  }
  return null;
}

// Unfinished and with a destination chosen.
export function inQueue(d: Download, queue: string): boolean {
  return (d.queue || mainQueue) === queue && !["done", "pending", "removing"].includes(d.status);
}

export function standing(q: QueueState, downloads: Download[], now = new Date()): string {
  const running = downloads.filter((d) => inQueue(d, q.id) && d.status === "downloading").length;
  if (q.running) return running ? `${running} running` : "Running";
  const next = nextStart(q, now);
  if (next) {
    const today = next.toDateString() === now.toDateString();
    const tomorrow = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
    const when = today
      ? ""
      : next.toDateString() === tomorrow.toDateString()
        ? ""
        : next.toLocaleDateString([], { weekday: "short" }) + " ";
    return `Starts ${when}at ${clock(q.schedule.start)}`;
  }
  return "Stopped";
}

export function startsIn(q: Queue, now = new Date()): string {
  const next = nextStart(q, now);
  return next
    ? "in " + duration(Math.max(60, Math.round((next.getTime() - now.getTime()) / 1000)))
    : "";
}

export function speedText(bytesPerSecond: number | undefined): string {
  if (!bytesPerSecond) return "No speed limit";
  return bytesPerSecond >= 1 << 20
    ? `Up to ${+(bytesPerSecond / (1 << 20)).toFixed(1)} MB/s`
    : `Up to ${Math.round(bytesPerSecond / 1024)} KB/s`;
}

export function whenDoneText(action: string | undefined): string {
  switch (action) {
    case "sleep":
      return "Sleep the PC when done";
    case "shutdown":
      return "Shut down the PC when done";
    case "quit":
      return "Quit Snag when done";
  }
  return "";
}

// Completes "Snag will … in 0:54".
export function countdownAction(c: Countdown): { will: string; now: string; keep: string } {
  switch (c.action) {
    case "sleep":
      return { will: "put this PC to sleep", now: "Sleep now", keep: "Don't sleep" };
    case "quit":
      return { will: "quit", now: "Quit now", keep: "Keep Snag open" };
  }
  return { will: "shut down this PC", now: "Shut down now", keep: "Don't shut down" };
}

export function retryText(retries: number | undefined, wait: number | undefined): string {
  const n = retries ?? 3;
  const seconds = wait ?? 5;
  if (n <= 0) return "Doesn't retry";
  const times = n === 1 ? "Retries once" : `Retries ${n} times`;
  if (seconds <= 0) return `${times}, straight away`;
  return `${times}, ${seconds === 1 ? "1s apart" : `${seconds}s apart`}`;
}
export function ordinal(n: number): string {
  const tens = n % 100;
  const suffix = tens >= 11 && tens <= 13 ? "th" : (["th", "st", "nd", "rd"][n % 10] ?? "th");
  return `${n}${suffix}`;
}

export function blankQueue(): Queue {
  return {
    id: "",
    name: "",
    atOnce: 2,
    schedule: { on: false, start: "02:00", stop: "", days: [] },
    speedLimit: 0,
    whenDone: "",
    naming: "",
    retries: 3,
    retryWait: 5,
  };
}
