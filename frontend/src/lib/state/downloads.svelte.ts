import type { Download } from "../api/client";
import { categories, queueOfView, subscriptionOfView, type FilterId } from "../filters";
import { categoryOf } from "../format";
import { inQueue, mainQueue } from "../queues";

const unfinished = new Set(["pending", "queued", "downloading", "paused", "pausing"]);

function inFilter(d: Download, filter: FilterId): boolean {
  const queue = queueOfView(filter);
  if (queue) return inQueue(d, queue);
  const subscription = subscriptionOfView(filter);
  if (subscription) return d.subscription === subscription;
  switch (filter) {
    case "all":
      return true;
    case "unfinished":
      return unfinished.has(d.status);
    case "finished":
      return d.status === "done";
    case "failed":
      return d.status === "error";
    default:
      return categoryOf(d).id === filter;
  }
}

// Newest first, as the Go side reports it.
export class Downloads {
  list = $state<Download[]>([]);

  counts = $derived.by(() => {
    const counts: Record<string, number> = {};
    for (const id of ["all", "unfinished", "finished", "failed", ...categories.map((c) => c.id)])
      counts[id] = this.list.filter((d) => inFilter(d, id as FilterId)).length;
    return counts as Record<FilterId, number>;
  });

  running = $derived(this.list.filter((d) => d.status === "downloading"));
  // ahead: per download, how many in its queue wait in front of it.
  ahead = $derived.by(() => {
    const groups: Record<string, { id: string; position: number }[]> = {};
    for (const d of this.list) {
      if (d.status !== "queued") continue;
      const queue = d.queue || mainQueue;
      (groups[queue] ??= []).push({ id: d.id, position: d.position ?? 0 });
    }
    const ahead: Record<string, number> = {};
    for (const group of Object.values(groups)) {
      group.sort((a, b) => a.position - b.position);
      group.forEach((item, i) => (ahead[item.id] = i));
    }
    return ahead;
  });
  speed = $derived(this.running.reduce((sum, d) => sum + (d.speed || 0), 0));

  get(id: string): Download | undefined {
    return this.list.find((d) => d.id === id);
  }

  upsert(d: Download): void {
    const i = this.list.findIndex((x) => x.id === d.id);
    if (i >= 0) this.list[i] = d;
    else this.list.unshift(d);
  }

  remove(id: string): void {
    this.list = this.list.filter((d) => d.id !== id);
  }

  // Newest first, or in line order for a queue.
  visible(filter: FilterId, search: string): Download[] {
    const q = search.trim().toLowerCase();
    const shown = this.list.filter(
      (d) =>
        inFilter(d, filter) &&
        (!q || d.name.toLowerCase().includes(q) || d.url.toLowerCase().includes(q)),
    );
    if (queueOfView(filter)) shown.sort((a, b) => (a.position ?? 0) - (b.position ?? 0));
    return shown;
  }
}
