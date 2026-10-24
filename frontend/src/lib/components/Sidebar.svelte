<script lang="ts">
  import Icon from "./Icon.svelte";
  import { filters, categories, queueView, subscriptionView, type Filter } from "../filters";
  import { size, subscriptionLine } from "../format";
  import { inQueue, mainQueue, standing } from "../queues";
  import { dragged, dragsDownloads } from "../drag";
  import { dragThreshold, layoutOf, offsets, reorder, type Layout } from "../sortable";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  let dropOn = $state("");

  // layout stays null until the press has moved far enough to be a drag.
  let drag = $state<{
    id: string;
    pointer: number;
    startY: number;
    layout: Layout | null;
    order: string[];
  } | null>(null);
  const slide = $derived(drag?.layout ? offsets(drag.order, drag.layout) : {});
  const rows: Record<string, HTMLElement> = {};
  let dragEnded = false; // the click that ends a drag isn't one

  function press(e: PointerEvent, id: string) {
    if (e.button !== 0 || id === mainQueue) return;
    drag = { id, pointer: e.pointerId, startY: e.clientY, layout: null, order: [] };
    try {
      (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
    } catch {
      // The pointer is gone; the drag still works while over the rows.
    }
  }

  function move(e: PointerEvent) {
    if (!drag || e.pointerId !== drag.pointer) return;
    if (!drag.layout) {
      if (Math.abs(e.clientY - drag.startY) < dragThreshold) return;
      const ids = snag.queues.map((q) => q.id).filter((id) => id !== mainQueue);
      drag.layout = layoutOf(ids.flatMap((id) => (rows[id] ? [[id, rows[id]] as const] : [])));
      drag.order = ids;
    }
    drag.order = reorder(drag.order, drag.id, e.clientY, drag.layout);
  }

  function release(e: PointerEvent, keep: boolean) {
    if (!drag || e.pointerId !== drag.pointer) return;
    const done = drag;
    drag = null; // first: the slide transition is off, so nothing jumps as the list reorders
    if (!done.layout) return;
    dragEnded = true;
    setTimeout(() => (dragEnded = false));
    const order = [mainQueue, ...done.order];
    if (keep && order.join() !== snag.queues.map((q) => q.id).join())
      void snag.arrangeQueues(order);
  }
  // Ticks every minute so "Starts at" stays current.
  let now = $state(new Date());
  $effect(() => {
    const timer = setInterval(() => (now = new Date()), 60_000);
    return () => clearInterval(timer);
  });
  const sidebarSubscriptions = 5;
  const shownSubscriptions = $derived(
    snag.subscriptions.length > sidebarSubscriptions + 1
      ? snag.subscriptions.slice(0, sidebarSubscriptions)
      : snag.subscriptions,
  );
  const allName = $derived(snag.subscriptions.length ? "All subscriptions" : "Follow a channel…");
  const pageOpen = $derived(snag.settingsOpen || snag.subscriptionsOpen);
  // Unreadable subscriptions not shown in the sidebar; shown ones say so themselves.
  const troubled = $derived(
    snag.subscriptions.filter((s) => s.error && !s.paused && !shownSubscriptions.includes(s))
      .length,
  );
  const allLine = $derived(
    troubled
      ? `${troubled} can't be read`
      : snag.subscriptions.length
        ? ""
        : "Or a playlist, or a podcast",
  );
  const running = $derived(snag.downloads.running.length);
  const rate = $derived(
    !snag.ready ? "Starting…" : running > 0 ? `${size(snag.downloads.speed) || "0 B"}/s` : "Idle",
  );
</script>

{#snippet entry(f: Filter, ink: string)}
  {@const count = snag.downloads.counts[f.id]}
  <button
    class:chosen={snag.filter === f.id && !pageOpen}
    style:--ink={ink}
    onclick={() => snag.show(f.id)}
  >
    <span class="glyph"><Icon name={f.icon} size={17} /></span>
    <span class="label">{f.label}</span>
    {#if count}<span class="count numbers">{count}</span>{/if}
  </button>
{/snippet}

<nav aria-label="Downloads">
  <div class="brand">
    <span class="logo"><Icon name="download" size={17} stroke={2.4} /></span>
    Snag
  </div>

  <h2>Downloads</h2>
  {#each filters as f (f.id)}
    {@render entry(f, "var(--text-dim)")}
  {/each}

  <div class="heading">
    <h2>Queues</h2>
    <button class="add" aria-label="New queue" onclick={() => (snag.queueSheet = { queue: null })}>
      <Icon name="plus" size={15} />
    </button>
  </div>
  {#each snag.queues as q (q.id)}
    {@const view = queueView(q.id)}
    {@const count = snag.downloads.list.filter((d) => inQueue(d, q.id)).length}
    {@const line = standing(q, snag.downloads.list, now)}
    <button
      class="queue"
      class:chosen={snag.filter === view && !pageOpen}
      class:drop={dropOn === q.id}
      class:sliding={!!drag?.layout}
      class:dragged={drag?.layout && drag.id === q.id}
      style:transform={slide[q.id] ? `translateY(${slide[q.id]}px)` : undefined}
      aria-label="{q.name}: {line}"
      bind:this={rows[q.id]}
      onclick={() => {
        if (!dragEnded) snag.show(view);
      }}
      oncontextmenu={(e) => {
        e.preventDefault();
        snag.openQueueMenu(q.id, e.clientX, e.clientY);
      }}
      onpointerdown={(e) => press(e, q.id)}
      onpointermove={move}
      onpointerup={(e) => release(e, true)}
      onpointercancel={(e) => release(e, false)}
      ondragover={(e) => {
        if (!dragsDownloads(e)) return;
        e.preventDefault();
        dropOn = q.id;
      }}
      ondragleave={() => dropOn === q.id && (dropOn = "")}
      ondrop={(e) => {
        dropOn = "";
        const ids = dragged(e);
        if (ids.length) {
          e.preventDefault();
          void snag.moveTo(ids, q.id);
        }
      }}
    >
      <span class="glyph state">
        {#if drag?.layout && drag.id === q.id}
          <Icon name="grip" size={17} />
        {:else if q.running}
          <span class="dot"></span>
        {:else if q.schedule?.on}
          <Icon name="clock" size={17} />
        {:else}
          <Icon name="pause" size={17} />
        {/if}
      </span>
      <span class="label">
        <span class="name">{q.name}</span>
        <span class="line" class:on={q.running}>{line}</span>
      </span>
      {#if count}<span class="count numbers">{count}</span>{/if}
    </button>
  {/each}

  <div class="heading">
    <h2>Subscriptions</h2>
    <button class="add" aria-label="Subscribe to…" onclick={() => snag.openSubscriptions(true)}>
      <Icon name="plus" size={15} />
    </button>
  </div>
  {#each shownSubscriptions as s (s.id)}
    {@const view = subscriptionView(s.id)}
    {@const line = subscriptionLine(s, now.getTime())}
    {@const count = snag.downloads.list.filter((d) => d.subscription === s.id).length}
    <button
      class="queue subscription"
      class:chosen={snag.filter === view && !pageOpen}
      aria-label="{s.title}: {line}"
      title={s.error || s.source}
      onclick={() => snag.show(view)}
      oncontextmenu={(e) => {
        e.preventDefault();
        snag.openSubscriptionMenu(s.id, e.clientX, e.clientY);
      }}
    >
      <span class="glyph state" class:bad={!!s.error && !s.paused}>
        <Icon name={s.paused ? "pause" : s.error ? "failed" : "subscribe"} size={17} />
      </span>
      <span class="label">
        <span class="name">{s.title}</span>
        <span class="line" class:on={s.checking || s.last > 0} class:bad={!!s.error && !s.paused}
          >{line}</span
        >
      </span>
      {#if count}<span class="count numbers">{count}</span>{/if}
    </button>
  {/each}
  <button
    class="queue subscription"
    class:chosen={snag.subscriptionsOpen}
    aria-label={allLine ? `${allName}: ${allLine}` : allName}
    onclick={() => snag.openSubscriptions()}
  >
    <span class="glyph state" class:bad={troubled > 0}>
      <Icon name={troubled > 0 ? "failed" : "settings"} size={17} />
    </span>
    <span class="label">
      <span class="name">{allName}</span>
      {#if allLine}<span class="line" class:bad={troubled > 0}>{allLine}</span>{/if}
    </span>
    {#if snag.subscriptions.length > shownSubscriptions.length}<span class="count numbers"
        >{snag.subscriptions.length}</span
      >{/if}
  </button>

  <h2>Categories</h2>
  {#each categories as c (c.id)}
    {@render entry(c, c.ink)}
  {/each}

  <div class="speed">
    <span class="gauge" class:on={running > 0}><Icon name="speed" size={20} /></span>
    <div>
      <div class="rate numbers">{rate}</div>
      <div class="running">
        {running > 0
          ? `${running} ${running === 1 ? "download" : "downloads"} running`
          : "Nothing downloading"}
      </div>
    </div>
  </div>
</nav>

<style>
  nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-3);
    background: var(--sidebar);
    border-right: 1px solid var(--border);
    overflow-y: auto;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 6px 10px;
    font-size: 17px;
    font-weight: 600;
  }

  .logo {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border-radius: var(--radius);
    background: var(--accent-strong);
    color: white;
  }

  h2 {
    margin: 0;
    padding: 14px 10px 6px;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.6px;
    text-transform: uppercase;
  }

  button {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 34px;
    padding: 0 10px;
    border: 0;
    border-radius: 7px;
    background: var(--sidebar);
    color: var(--text-dim);
    text-align: left;
    cursor: pointer;
    transition: background-color var(--fast);
  }

  button:hover {
    background: var(--surface-hover);
  }

  .glyph {
    display: grid;
    color: var(--ink);
  }

  .label {
    flex: 1;
    min-width: 0;
  }

  .heading {
    display: flex;
    align-items: center;
    padding-right: 4px;
  }

  .heading h2 {
    flex: 1;
  }

  .add {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    margin-top: 8px;
    padding: 0;
  }

  .queue {
    height: 42px;
  }

  .queue .label {
    display: flex;
    flex-direction: column;
    line-height: 1.25;
  }

  .name,
  .line {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .line {
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 400;
  }

  .line.on {
    color: var(--good);
  }

  .line.bad,
  .state.bad {
    color: var(--bad);
  }

  .state {
    width: 17px;
    place-items: center;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 4px;
    background: var(--good);
  }

  .queue.drop {
    outline: 2px dashed var(--accent-strong);
    outline-offset: -2px;
    background: var(--accent-soft);
  }

  .queue.sliding {
    transition: transform var(--medium) ease-out;
  }

  .queue.dragged {
    position: relative;
    z-index: 1;
    box-shadow: inset 0 0 0 1px var(--accent-strong);
    background: var(--accent-soft);
    cursor: grabbing;
  }

  .count {
    color: var(--text-faint);
    font-size: var(--text-sm);
  }

  button.chosen {
    background: var(--surface-active);
    color: var(--text);
    font-weight: 600;
  }

  .chosen .glyph,
  .chosen .count {
    color: var(--accent);
  }

  .speed {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: auto;
    height: 58px;
    padding: 0 12px;
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  .gauge {
    display: grid;
    color: var(--text-faint);
  }

  .gauge.on {
    color: var(--accent);
  }

  .rate {
    font-size: var(--text-md);
    font-weight: 600;
  }

  .running {
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
</style>
