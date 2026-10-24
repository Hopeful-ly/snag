<script lang="ts">
  import { tooltip } from "../tooltip.svelte";

  let width = $state(0);
  const position = $derived.by(() => {
    const r = tooltip.rect;
    if (!r) return null;
    const left = Math.max(
      6,
      Math.min(r.left + r.width / 2 - width / 2, window.innerWidth - width - 6),
    );
    const below = r.bottom + 6;
    return { left, top: below + 28 > window.innerHeight ? r.top - 32 : below };
  });
</script>

{#if position}
  <div
    class="tooltip"
    role="tooltip"
    bind:clientWidth={width}
    style:left="{position.left}px"
    style:top="{position.top}px"
  >
    {tooltip.text}
  </div>
{/if}

<style>
  .tooltip {
    position: fixed;
    z-index: 1000;
    padding: 4px 8px;
    border: 1px solid var(--border-strong);
    border-radius: 6px;
    background: var(--tooltip);
    font-size: var(--text-sm);
    white-space: nowrap;
    pointer-events: none;
  }
</style>
