<script lang="ts" generics="T extends MenuChoice">
  import Icon from "./Icon.svelte";
  import type { MenuChoice } from "../state/actions";

  let {
    label,
    x,
    y,
    items,
    onpick,
    onclose,
  }: {
    label: string;
    x: number;
    y: number;
    items: (T | "separator")[];
    onpick: (item: T) => void;
    onclose: () => void;
  } = $props();
  let card: HTMLElement | undefined = $state();
  let size = $state({ width: 236, height: 0 });

  const place = $derived.by(() => {
    const left = Math.max(8, Math.min(x, window.innerWidth - size.width - 8));
    const top = y + size.height > window.innerHeight - 8 ? Math.max(8, y - size.height - 8) : y;
    return { left, top };
  });

  $effect(() => {
    card?.querySelector<HTMLElement>("[role=menuitem]:not(:disabled)")?.focus();
  });
</script>

<div
  class="backdrop"
  role="presentation"
  onpointerdown={onclose}
  oncontextmenu={(e) => {
    e.preventDefault();
    onclose();
  }}
></div>
<div
  class="menu"
  role="menu"
  tabindex="-1"
  aria-label={label}
  bind:this={card}
  bind:clientHeight={size.height}
  style:left="{place.left}px"
  style:top="{place.top}px"
  onkeydown={(e) => {
    if (e.key === "Escape") {
      onclose();
      e.stopPropagation();
    }
  }}
>
  {#each items as item, i (i)}
    {#if item === "separator"}
      <div class="separator" role="separator"></div>
    {:else}
      <button
        role="menuitem"
        class:danger={item.danger}
        disabled={item.disabled}
        onclick={() => {
          onpick(item); // while what the menu is about is still there
          onclose();
        }}
      >
        <Icon name={item.icon} size={16} />
        {item.label}
      </button>
    {/if}
  {/each}
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 800;
  }

  .menu {
    position: fixed;
    z-index: 801;
    width: 236px;
    padding: 6px;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--raised);
    animation: open var(--fast) ease-out;
  }

  @keyframes open {
    from {
      opacity: 0;
      transform: scale(0.97);
    }
  }

  button {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    height: 34px;
    padding: 0 10px;
    border: 0;
    border-radius: 6px;
    background: var(--raised);
    color: var(--text);
    text-align: left;
    cursor: pointer;
    transition: background-color var(--fast);
  }

  button :global(svg) {
    color: var(--text-dim);
  }

  button:hover:not(:disabled),
  button:focus-visible {
    outline: none;
    background: var(--raised-hover);
  }

  button:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .danger,
  .danger :global(svg) {
    color: var(--bad);
  }

  .danger:hover:not(:disabled),
  .danger:focus-visible {
    background: var(--bad-soft);
  }

  .separator {
    height: 1px;
    margin: 4px 6px;
    background: var(--border);
  }
</style>
