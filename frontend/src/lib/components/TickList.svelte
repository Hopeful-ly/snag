<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "./Icon.svelte";
  import { toggle } from "../playlist";

  // order: the shown rows as indexes into picks, top first.
  let {
    picks = $bindable(),
    order,
    label,
    name,
    busy = false,
    row,
  }: {
    picks: boolean[];
    order: number[];
    label: (i: number) => string;
    name: string;
    busy?: boolean;
    row: Snippet<[number]>;
  } = $props();

  let anchor: number | null = null;
  let list: HTMLElement | undefined = $state();
  let sweep: { on: boolean; last: number; pointer: number; y: number } | null = null;
  let scrolling = 0; // rAF id while auto-scrolling past an edge
  const edge = 28; // px from the list's top or bottom where auto-scroll starts

  // Ranges cover only the rows shown, so rows a filter hides keep their state.
  function between(from: number, to: number): number[] {
    const a = order.indexOf(from);
    const b = order.indexOf(to);
    if (a < 0 || b < 0) return [to];
    return order.slice(Math.min(a, b), Math.max(a, b) + 1);
  }

  function set(indexes: number[], on: boolean) {
    const these = new Set(indexes);
    picks = picks.map((picked, i) => (these.has(i) ? on : picked));
  }

  function pick(i: number, range: boolean) {
    if (range && anchor !== null) set(between(anchor, i), !picks[i]);
    else picks = toggle(picks, i, null);
    anchor = i;
  }

  function press(e: PointerEvent, i: number) {
    if (e.button !== 0 || busy) return;
    e.preventDefault();
    if (e.shiftKey) {
      pick(i, true);
      return;
    }
    const on = !picks[i];
    set([i], on);
    anchor = i;
    sweep = { on, last: i, pointer: e.pointerId, y: e.clientY };
    scrolling = requestAnimationFrame(scrollWhileHeld);
  }

  function sweepTo(e: PointerEvent) {
    if (!sweep || e.pointerId !== sweep.pointer) return;
    sweep.y = e.clientY;
    paintAt(e.clientY);
  }

  function endSweep() {
    sweep = null;
    cancelAnimationFrame(scrolling);
  }

  function paintAt(y: number) {
    const first = list?.querySelector<HTMLElement>(".entry");
    if (!sweep || !first || !order.length) return;
    const box = first.getBoundingClientRect();
    const place = Math.max(0, Math.min(order.length - 1, Math.floor((y - box.top) / box.height)));
    const i = order[place] ?? sweep.last;
    if (i === sweep.last) return;
    set(between(sweep.last, i), sweep.on);
    sweep.last = anchor = i;
  }

  function scrollWhileHeld() {
    if (!sweep || !list) return;
    const box = list.getBoundingClientRect();
    const past =
      sweep.y < box.top + edge
        ? sweep.y - box.top - edge
        : sweep.y > box.bottom - edge
          ? sweep.y - box.bottom + edge
          : 0;
    if (past) {
      list.scrollTop += Math.max(-14, Math.min(14, past / 3));
      paintAt(Math.max(box.top + 1, Math.min(box.bottom - 1, sweep.y)));
    }
    scrolling = requestAnimationFrame(scrollWhileHeld);
  }
</script>

<svelte:window onpointermove={sweepTo} onpointerup={endSweep} onpointercancel={endSweep} />

<div class="entries" role="group" aria-label={name} bind:this={list}>
  {#each order as i (i)}
    <button
      class="entry"
      role="checkbox"
      aria-checked={picks[i]}
      aria-label={label(i)}
      disabled={busy}
      onpointerdown={(e) => press(e, i)}
      onclick={(e) => e.detail === 0 && pick(i, e.shiftKey)}
    >
      <span class="box" class:on={picks[i]}
        >{#if picks[i]}<Icon name="check" size={13} />{/if}</span
      >
      {@render row(i)}
    </button>
  {/each}
</div>

<style>
  .entries {
    user-select: none;
    flex: 1 0 auto;
    height: 0;
    min-height: 104px;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--field);
  }

  .entry {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    height: 34px;
    padding: 0 12px;
    border: 0;
    background: none;
    color: var(--text);
    font-size: var(--text-base);
    text-align: left;
    cursor: pointer;
    transition: background-color var(--fast);
  }

  .entry:hover:not(:disabled) {
    background: var(--surface-hover);
  }

  .entry[aria-checked="false"] :global(.name) {
    color: var(--text-dim);
  }

  .box {
    display: grid;
    place-items: center;
    flex: none;
    width: 16px;
    height: 16px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    color: white;
  }

  .box.on {
    border-color: var(--accent-strong);
    background: var(--accent-strong);
  }
</style>
