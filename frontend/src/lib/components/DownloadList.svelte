<script lang="ts">
  import { fade, fly } from "svelte/transition";
  import { flip } from "svelte/animate";
  import DownloadRow from "./DownloadRow.svelte";
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import type { Download } from "../api/client";
  import { dragged, dragsDownloads } from "../drag";
  import { queueOfView } from "../filters";
  import { useSnag } from "../state/context";

  // Past this many rows, sliding each one into place costs more than it shows.
  const animateUntil = 24;
  const snag = useSnag();
  const queue = $derived(queueOfView(snag.filter));
  // Drop position: before the visible download at this index.
  let dropAt = $state(-1);

  function over(e: DragEvent, index: number) {
    if (!queue || !dragsDownloads(e)) return;
    e.preventDefault();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    dropAt = e.clientY < r.top + r.height / 2 ? index : index + 1;
  }

  function drop(e: DragEvent) {
    const at = dropAt;
    dropAt = -1;
    const moving = dragged(e);
    if (!queue || at < 0 || !moving.length) return;
    e.preventDefault();
    const order = snag.visible.map((d) => d.id);
    const before = order.slice(0, at).filter((id) => !moving.includes(id));
    const after = order.slice(at).filter((id) => !moving.includes(id));
    const outside = moving.filter((id) => !order.includes(id));
    if (outside.length) void snag.moveTo(outside, queue);
    void snag.reorder(queue, [...before, ...moving, ...after]);
  }
  const empty = $derived.by(() => {
    if (!snag.ready) return { icon: "clock", text: "Starting…" } as const;
    if (snag.search) return { icon: "search", text: `Nothing matches “${snag.search}”` } as const;
    if (snag.filter === "failed") return { icon: "done", text: "No failed downloads" } as const;
    if (snag.filter === "all") return { icon: "inbox", text: "No downloads yet" } as const;
    if (snag.queue)
      return { icon: "queue", text: `Nothing waiting in ${snag.queue.name}` } as const;
    return { icon: "inbox", text: "Nothing here" } as const;
  });
  const firstRun = $derived(snag.ready && snag.filter === "all" && !snag.search);
</script>

{#snippet item(d: Download, i: number)}
  <div
    role="presentation"
    class:drop-before={dropAt === i}
    class:drop-after={dropAt === i + 1 && i === snag.visible.length - 1}
    ondragover={(e) => over(e, i)}
  >
    <DownloadRow {d} place={queue ? i + 1 : 0} />
  </div>
{/snippet}

{#if snag.visible.length}
  <div
    class="list"
    class:plain={snag.visible.length > animateUntil}
    role="listbox"
    tabindex="-1"
    aria-label={snag.queue ? `${snag.queue.name}, in order` : "Downloads"}
    ondragleave={(e) => {
      if (!(e.currentTarget as Element).contains(e.relatedTarget as Node)) dropAt = -1;
    }}
    ondrop={drop}
    ondragend={() => (dropAt = -1)}
  >
    {#if snag.visible.length > animateUntil}
      {#each snag.visible as d, i (d.id)}
        {@render item(d, i)}
      {/each}
    {:else}
      {#each snag.visible as d, i (d.id)}
        <div
          animate:flip={{ duration: 200 }}
          in:fly={{ x: -12, duration: 200 }}
          out:fade={{ duration: 160 }}
        >
          {@render item(d, i)}
        </div>
      {/each}
    {/if}
    {#if queue && snag.visible.length > 1}
      <p class="hint">
        Drag to change the order. {snag.queue?.atOnce === 1
          ? "The first one runs"
          : `The first ${snag.queue?.atOnce ?? 1} run together`}; the next starts as one finishes.
        {#if snag.queue?.retries}
          A failed one is tried again {snag.queue.retries}
          {snag.queue.retries === 1 ? "time" : "times"} first.
        {/if}
      </p>
    {/if}
  </div>
{:else}
  <div class="empty">
    <Icon name={empty.icon} size={42} stroke={1.6} />
    <h2>{empty.text}</h2>
    {#if snag.queue}
      <p>Drag downloads onto it in the sidebar, or pick it when saving one.</p>
    {/if}
    {#if firstRun}
      <p>Use the Download button on a video in your browser, or add a link here.</p>
      <IconButton
        icon="plus"
        label="Add a link"
        kind="primary"
        onclick={() => (snag.addOpen = true)}
      />
    {/if}
  </div>
{/if}

<style>
  .list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    flex: 1;
    min-height: 0;
    padding: 0 16px 12px;
    overflow-y: auto;
  }

  .drop-before,
  .drop-after {
    position: relative;
  }

  .drop-before::before,
  .drop-after::after {
    position: absolute;
    right: 10px;
    left: 10px;
    height: 3px;
    border-radius: 2px;
    background: var(--accent-strong);
    content: "";
  }

  .drop-before::before {
    top: -6px;
  }

  .drop-after::after {
    bottom: -6px;
  }

  .hint {
    margin: 4px 6px 0;
    color: var(--text-faint);
    font-size: var(--text-sm);
  }

  /* Off-screen rows of a long list skip layout. A row is 72px tall. */
  .list.plain > div {
    content-visibility: auto;
    contain-intrinsic-size: auto 72px;
  }

  .list::-webkit-scrollbar {
    width: 6px;
  }

  .list::-webkit-scrollbar-thumb {
    border-radius: 3px;
    background: var(--border-strong);
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    flex: 1;
    color: var(--text-faint);
  }

  h2 {
    margin: 0;
    color: var(--text-dim);
    font-size: 15px;
    font-weight: 500;
  }

  p {
    margin: 0;
    font-size: var(--text-sm);
  }
</style>
