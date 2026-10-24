import type { IconName } from "./icons";

export type CategoryId = "video" | "music" | "archives" | "programs" | "documents" | "other";
export type FilterId =
  "all" | "unfinished" | "finished" | "failed" | CategoryId | QueueView | SubscriptionView;

export type QueueView = `queue:${string}`;

export type SubscriptionView = `subscription:${string}`;

export function subscriptionView(id: string): SubscriptionView {
  return `subscription:${id}`;
}

export function subscriptionOfView(id: FilterId): string {
  return id.startsWith("subscription:") ? id.slice("subscription:".length) : "";
}

export function queueView(id: string): QueueView {
  return `queue:${id}`;
}

export function queueOfView(id: FilterId): string {
  return id.startsWith("queue:") ? id.slice("queue:".length) : "";
}

export interface Filter {
  id: FilterId;
  label: string; // in the sidebar
  title: string; // over the list
  icon: IconName;
}

export interface Category extends Filter {
  id: CategoryId;
  tile: string; // the colour of its downloads' tiles
  ink: string; // and of the icon on them
}

export const filters: Filter[] = [
  { id: "all", label: "All", title: "All downloads", icon: "list" },
  { id: "unfinished", label: "Unfinished", title: "Unfinished", icon: "downloading" },
  { id: "finished", label: "Finished", title: "Finished", icon: "done" },
  { id: "failed", label: "Failed", title: "Failed", icon: "failed" },
];

export const categories: Category[] = [
  { id: "video", label: "Video", title: "Video", icon: "video", tile: "#3b2a1f", ink: "#fb923c" },
  { id: "music", label: "Music", title: "Music", icon: "music", tile: "#2d2445", ink: "#c4b5fd" },
  {
    id: "archives",
    label: "Archives",
    title: "Archives",
    icon: "archive",
    tile: "#3a321a",
    ink: "#facc15",
  },
  {
    id: "programs",
    label: "Programs",
    title: "Programs",
    icon: "program",
    tile: "#16332f",
    ink: "#5eead4",
  },
  {
    id: "documents",
    label: "Documents",
    title: "Documents",
    icon: "document",
    tile: "#3a1f24",
    ink: "#fda4af",
  },
  { id: "other", label: "Other", title: "Other", icon: "file", tile: "#262b33", ink: "#cbd5e1" },
];

export function titleOf(id: FilterId): string {
  return [...filters, ...categories].find((f) => f.id === id)?.title ?? "";
}
