import type { Download, Player, QueueState } from "../api/client";
import { inQueue, mainQueue } from "../queues";
import type { IconName } from "../icons";
import type { SubscriptionAction } from "./snag.svelte";

export type Action =
  | "open"
  | "watch"
  | "watchWith"
  | "copyAddress"
  | "choose"
  | "where"
  | "reveal"
  | "copy"
  | "pause"
  | "resume"
  | "remove"
  | "delete"
  | "startNow"
  | "toFront"
  | "move"
  | "newQueue"
  | "check"
  | "checkSheet"
  | "redownload"
  | "details";

export interface MenuChoice {
  icon: IconName;
  label: string;
  danger?: boolean;
  disabled?: boolean;
}

interface MenuItem extends MenuChoice {
  action: Action;
  player?: string; // watchWith: which player
  queue?: string; // move: to which queue
}

type MenuEntry = MenuItem | "separator";

export type QueueAction = "start" | "stop" | "edit" | "reverse" | "up" | "down" | "delete";

interface QueueMenuItem extends MenuChoice {
  action: QueueAction;
}

// place is the queue's index; Main is 0 and stays first.
export function queueMenuFor(
  q: QueueState,
  place: number,
  count: number,
): (QueueMenuItem | "separator")[] {
  const run: QueueMenuItem = q.running
    ? { icon: "stop", label: "Stop", action: "stop" }
    : { icon: "play", label: "Start now", action: "start" };
  const edit: QueueMenuItem = { icon: "queue", label: "Edit…", action: "edit" };
  const reverse: QueueMenuItem = { icon: "reverse", label: "Reverse", action: "reverse" };
  if (q.id === mainQueue) return [run, edit, reverse];
  return [
    run,
    edit,
    reverse,
    "separator",
    { icon: "up", label: "Move up", action: "up", disabled: place <= 1 },
    { icon: "down", label: "Move down", action: "down", disabled: place >= count - 1 },
    "separator",
    { icon: "trash", label: "Delete…", action: "delete", danger: true },
  ];
}

export function primaryOf(
  d: Download,
  queueRunning = true,
): { action: Action; icon: IconName; tip: string } | null {
  if (d.where) return { action: "where", icon: "retry", tip: "Try again" };
  if (d.status === "queued" && !queueRunning)
    return { action: "startNow", icon: "play", tip: "Start this one now" };
  switch (d.status) {
    case "downloading":
    case "queued":
      return { action: "pause", icon: "pause", tip: "Pause" };
    case "pausing":
      return { action: "pause", icon: "pause", tip: "Pausing…" };
    case "paused":
      return { action: "resume", icon: "play", tip: "Resume" };
    case "error":
      return { action: "resume", icon: "retry", tip: "Try again" };
    case "pending":
      return { action: "choose", icon: "save", tip: "Choose where to save" };
    case "done":
      return { action: "open", icon: "open", tip: "Open file" };
  }
  return null;
}

export function doubleClickOf(d: Download): Action {
  if (d.status === "done") return "open";
  if (d.status === "pending") return "choose";
  if (d.where) return "where";
  return "reveal";
}

// players: default first. queues: Main first.
export function menuFor(
  d: Download,
  players: Player[] = [],
  queues: QueueState[] = [],
): MenuEntry[] {
  const items: MenuEntry[] = [];
  if (d.status === "done") items.push({ icon: "open", label: "Open", action: "open" });
  items.push({ icon: "list", label: "Details", action: "details" });
  if (d.watch === "ready") {
    const label = players[0] ? `Watch in ${players[0].name}` : "Watch now";
    items.push({ icon: "play", label, action: "watch" });
    for (const p of players.slice(1)) {
      items.push({ icon: "play", label: `Watch in ${p.name}`, action: "watchWith", player: p.id });
    }
    items.push({ icon: "link", label: "Copy stream address", action: "copyAddress" });
  }
  if (d.status === "pending")
    items.push({ icon: "save", label: "Choose where to save…", action: "choose" });
  items.push({
    icon: "folder",
    label: "Show in folder",
    action: "reveal",
    disabled: d.status === "pending",
  });
  items.push({ icon: "link", label: "Copy link", action: "copy" });
  if (d.status === "done")
    items.push({ icon: "check", label: "Check checksum…", action: "checkSheet" });
  if (d.status === "downloading" || d.status === "queued")
    items.push("separator", { icon: "pause", label: "Pause", action: "pause" });
  if (d.status === "paused")
    items.push("separator", { icon: "play", label: "Resume", action: "resume" });
  if (d.where) items.push("separator", { icon: "retry", label: "Try again…", action: "where" });
  else if (d.status === "error")
    items.push("separator", { icon: "retry", label: "Try again", action: "resume" });
  const queue = d.queue || mainQueue;
  // Queue items show once there is a queue besides Main, so menus stay short for everyone else.
  if (inQueue(d, queue) && queues.length > 1) {
    items.push("separator");
    const running = queues.find((q) => q.id === queue)?.running ?? true;
    if (d.status === "queued" || (!running && ["paused", "error"].includes(d.status)))
      items.push({ icon: "play", label: "Start now", action: "startNow" });
    if (d.status === "queued" || d.status === "paused")
      items.push({ icon: "front", label: "First in its queue", action: "toFront" });
    for (const q of queues)
      if (q.id !== queue)
        items.push({ icon: "queue", label: `Move to ${q.name}`, action: "move", queue: q.id });
    items.push({ icon: "plus", label: "Move to a new queue…", action: "newQueue" });
  }
  items.push("separator");
  if (d.status === "done") {
    items.push({ icon: "x", label: "Remove from list", action: "remove" });
    items.push({ icon: "trash", label: "Delete file", action: "delete", danger: true });
  } else {
    items.push({ icon: "trash", label: "Cancel and remove", action: "remove", danger: true });
  }
  return items;
}

interface SubscriptionMenuItem extends MenuChoice {
  action: SubscriptionAction;
}

export function subscriptionMenuFor(s: {
  paused: boolean;
  checking: boolean;
}): (SubscriptionMenuItem | "separator")[] {
  return [
    { icon: "retry", label: "Check now", action: "check", disabled: s.checking },
    s.paused
      ? { icon: "play", label: "Resume", action: "resume" }
      : { icon: "pause", label: "Pause", action: "pause" },
    { icon: "settings", label: "Edit…", action: "edit" },
    "separator",
    { icon: "list", label: "Show its downloads", action: "downloads" },
    { icon: "link", label: "Open its page", action: "page" },
    "separator",
    { icon: "trash", label: "Unsubscribe", action: "remove", danger: true },
  ];
}
