<script lang="ts">
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import ProgressBar from "./ProgressBar.svelte";
  import type { Download } from "../api/client";
  import { startDrag } from "../drag";
  import { categoryOf, checkAbout, duration, meta, percent } from "../format";
  import { clock, inQueue, mainQueue, ordinal, scheduleText } from "../queues";
  import { doubleClickOf, primaryOf } from "../state/actions";
  import { useSnag } from "../state/context";

  // place: number in the shown queue's line, 0 elsewhere.
  let { d, place = 0 }: { d: Download; place?: number } = $props();
  const snag = useSnag();
  const queue = $derived(snag.queueNamed(d.queue));
  const queued = $derived(inQueue(d, d.queue || mainQueue));
  const waiting = $derived.by(() => {
    if (d.status !== "queued" || !queue) return undefined;
    const where = queue.id === mainQueue && snag.queues.length < 2 ? "" : ` in ${queue.name}`;
    if (!queue.running)
      return scheduleText(queue)
        ? `Waits for ${clock(queue.schedule.start)}${where}`
        : `Waiting${where}: the queue is stopped`;
    const ahead = snag.downloads.ahead[d.id] ?? 0;
    return `${ordinal(ahead + 1)} in line${where}`;
  });

  const category = $derived(categoryOf(d));
  const pct = $derived(percent(d));
  const primary = $derived(primaryOf(d, queue?.running ?? true));
  const busyAction = $derived(snag.busyActionOf(d.id));
  const working = $derived(busyAction !== "" || d.status === "pausing" || d.status === "removing");
  const selected = $derived(snag.selected === d.id);
  const watchLabel = $derived(
    d.watch === "ready"
      ? "Watch"
      : d.watchIn != null
        ? `Watch in ${duration(Math.max(1, d.watchIn))}`
        : "Watch soon",
  );
  // Keep the extension visible when the name is truncated.
  const tail = $derived(d.name.length > 24 ? d.name.slice(-10) : "");
  const head = $derived(tail ? d.name.slice(0, -10) : d.name);
  const badge = $derived(
    d.status === "done"
      ? "done"
      : d.status === "error"
        ? "failed"
        : d.status === "pending"
          ? "clock"
          : null,
  );

  const check = $derived(d.status === "done" ? d.check : null);
  const canCheck = $derived(!!check && (check.result === "" || check.result === "failed"));
  const tip = $derived(
    d.note ||
      (d.where ? d.error : undefined) ||
      (check && check.result !== "looking" ? checkAbout(check) : undefined),
  );

  function menu(e: MouseEvent) {
    e.preventDefault();
    snag.openMenu(d.id, e.clientX, e.clientY);
  }
</script>

<div
  class="row {d.status}"
  class:noted={d.status === "done" && d.note !== ""}
  class:matched={check?.result === "match"}
  class:mismatched={check?.result === "mismatch" || check?.result === "failed"}
  class:selected
  role="option"
  tabindex="-1"
  aria-selected={selected}
  aria-label={d.name}
  onclick={() => {
    snag.selected = d.id;
    if (d.status === "pending") void snag.act(d.id, "choose");
    else if (d.where) void snag.act(d.id, "where");
    else snag.detailsFor = d.id;
  }}
  ondblclick={() => !working && snag.act(d.id, doubleClickOf(d))}
  draggable={queued && !working}
  ondragstart={(e) => startDrag(e, [d.id])}
  oncontextmenu={menu}
  onkeydown={(e) => e.key === "Enter" && !working && snag.act(d.id, doubleClickOf(d))}
>
  {#if place}
    <span class="grip" aria-hidden="true"><Icon name="grip" size={16} stroke={2.6} /></span>
    <span class="place numbers" aria-label="{ordinal(place)} in line">{place}</span>
  {/if}
  <div class="tile" style:background={category.tile} style:color={category.ink}>
    <Icon name={category.icon} size={20} />
    {#if badge}<span class="badge {badge}"><Icon name={badge} size={14} stroke={2.3} /></span>{/if}
  </div>

  <div class="body">
    <div class="top">
      <span class="name" title={d.path || d.url}
        ><span class="head">{head}</span>{#if tail}<span class="tail">{tail}</span>{/if}</span
      >
      {#if pct >= 0 && d.status !== "done" && d.status !== "pending"}<span class="pct numbers"
          >{pct}%</span
        >{/if}
    </div>
    {#if d.status !== "done" && d.status !== "pending"}
      <ProgressBar
        value={pct >= 0 ? pct / 100 : 0}
        indeterminate={d.status === "downloading" && pct < 0}
        color={d.status === "error"
          ? "var(--bad)"
          : d.status === "downloading"
            ? "var(--accent)"
            : "var(--text-faint)"}
      />
    {/if}
    <div class="meta numbers" title={tip}>
      {meta(d, Date.now(), waiting)}
    </div>
  </div>

  <div class="actions">
    {#if d.watch}
      <IconButton
        icon="play"
        label={watchLabel}
        tip={d.watch !== "ready"
          ? "It can be watched once its start has downloaded"
          : snag.players[0]
            ? `Watch it in ${snag.players[0].name} while it downloads`
            : "Watch it while it downloads"}
        kind={d.watch === "ready" ? "primary" : "plain"}
        busy={busyAction === "watch"}
        disabled={working || d.watch !== "ready"}
        onclick={() => snag.act(d.id, "watch")}
      />
    {/if}
    {#if primary}
      <IconButton
        icon={primary.icon}
        tip={primary.tip}
        kind={d.status === "pending" || d.where ? "primary" : "plain"}
        busy={busyAction === primary.action || d.status === "pausing"}
        disabled={working}
        onclick={() => snag.act(d.id, primary.action)}
      />
    {/if}
    {#if check && canCheck}
      <IconButton
        icon="check"
        label="Check"
        tip="Check the file against its {check.algo}: Snag reads the whole file"
        kind="primary"
        busy={busyAction === "check"}
        disabled={working}
        onclick={() => snag.act(d.id, "check")}
      />
    {:else if check?.result === "mismatch"}
      <IconButton
        icon="retry"
        label="Download again"
        tip="Download it again: the new file replaces this one once it is complete"
        kind="primary"
        busy={busyAction === "redownload"}
        disabled={working}
        onclick={() => snag.act(d.id, "redownload")}
      />
    {/if}
    {#if d.status !== "pending"}
      <IconButton
        icon="folder"
        tip="Show in folder"
        busy={busyAction === "reveal"}
        disabled={working}
        onclick={() => snag.act(d.id, "reveal")}
      />
    {/if}
    <IconButton
      icon="x"
      tip={d.status === "done" ? "Remove from list (keeps the file)" : "Cancel and remove"}
      kind="ghost"
      busy={busyAction === "remove" || d.status === "removing"}
      disabled={working}
      onclick={() => snag.act(d.id, "remove")}
    />
    <IconButton
      icon="more"
      tip="More"
      kind="ghost"
      disabled={d.status === "removing"}
      onclick={(e) => {
        const r = (e.target as Element).closest("button")?.getBoundingClientRect();
        if (!r) return;
        snag.openMenu(d.id, r.right - 220, r.bottom + 4);
      }}
    />
  </div>
</div>

<style>
  .row {
    --ghost: var(--surface);
    display: flex;
    align-items: center;
    gap: 14px;
    height: 72px;
    padding: 0 12px 0 14px;
    border-radius: var(--radius-lg);
    background: var(--surface);
    transition:
      background-color var(--fast),
      opacity var(--medium);
  }

  .row:hover {
    --ghost: var(--surface-hover);
    background: var(--surface-hover);
  }

  .row.selected {
    --ghost: var(--surface-active);
    background: var(--surface-active);
  }

  .row.removing {
    opacity: 0.5;
  }

  .row[draggable="true"] {
    cursor: grab;
  }

  .grip {
    display: grid;
    margin-right: -8px;
    color: var(--text-faint);
  }

  .place {
    width: 18px;
    color: var(--text-faint);
    font-size: var(--text-sm);
    font-weight: 600;
    text-align: center;
  }

  .tile {
    position: relative;
    display: grid;
    place-items: center;
    flex: none;
    width: 42px;
    height: 42px;
    border-radius: 11px;
  }

  .badge {
    position: absolute;
    right: -3px;
    bottom: -3px;
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    border-radius: 9px;
    background: var(--surface);
  }

  .row:hover .badge {
    background: var(--surface-hover);
  }

  .selected .badge {
    background: var(--surface-active);
  }

  .badge.done {
    color: var(--good);
  }

  .badge.failed {
    color: var(--bad);
  }

  .badge.clock {
    color: var(--warn);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1;
    min-width: 0;
  }

  .top {
    display: flex;
    gap: 10px;
  }

  .name {
    display: flex;
    flex: 1;
    min-width: 0;
    font-size: var(--text-md);
    font-weight: 500;
    white-space: pre;
  }

  .head {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tail {
    flex: none;
  }

  .pct {
    color: var(--text-faint);
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .downloading .pct {
    color: var(--accent);
  }

  .meta {
    overflow: hidden;
    color: var(--text-faint);
    font-size: var(--text-sm);
    white-space: pre;
    text-overflow: ellipsis;
  }

  .downloading .meta {
    color: var(--accent);
  }

  .done .meta {
    color: var(--text-dim);
  }

  .done.noted .meta {
    color: var(--warn);
  }

  .done.matched .meta {
    color: var(--good);
  }

  .done.mismatched .meta {
    color: var(--bad);
  }

  .pending .meta {
    color: var(--warn);
  }

  .error .meta {
    color: var(--bad);
  }

  .actions {
    display: flex;
    gap: 6px;
  }
</style>
