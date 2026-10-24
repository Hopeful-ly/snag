<script lang="ts">
  import Icon from "./Icon.svelte";
  import Spinner from "./Spinner.svelte";
  import { tip as tooltip } from "../tooltip.svelte";
  import type { IconName } from "../icons";

  let {
    icon,
    tip = "",
    label = "",
    kind = "plain",
    busy = false,
    active = false,
    disabled = false,
    size = 34,
    iconSize = 18,
    onclick,
  }: {
    icon: IconName;
    tip?: string;
    label?: string;
    kind?: "plain" | "primary" | "danger" | "ghost";
    busy?: boolean;
    active?: boolean;
    disabled?: boolean;
    size?: number;
    iconSize?: number;
    onclick?: (event: MouseEvent) => void;
  } = $props();
</script>

<button
  type="button"
  class={kind}
  class:active
  class:labelled={label !== ""}
  class:busy
  style:--size="{size}px"
  disabled={disabled || busy}
  aria-label={label || tip}
  aria-busy={busy}
  use:tooltip={tip}
  onclick={(e) => {
    e.stopPropagation();
    onclick?.(e);
  }}
>
  {#if busy}
    <Spinner size={iconSize - 2} />
  {:else}
    <Icon name={icon} size={iconSize} />
  {/if}
  {#if label}<span>{label}</span>{/if}
</button>

<style>
  button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    flex: none;
    width: var(--size);
    height: var(--size);
    padding: 0;
    border: 0;
    border-radius: var(--radius);
    background: var(--raised);
    color: var(--text-dim);
    font-weight: 600;
    cursor: pointer;
    transition:
      background-color var(--fast),
      color var(--fast),
      opacity var(--fast),
      transform 90ms;
  }

  button.labelled {
    width: auto;
    padding: 0 12px;
  }

  button:hover:not(:disabled) {
    background: var(--raised-hover);
    color: var(--text);
  }

  button:active:not(:disabled) {
    transform: scale(0.94);
  }

  button:disabled {
    cursor: default;
  }

  button:disabled:not(.busy) {
    opacity: 0.4;
  }

  .active {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .active:hover:not(:disabled) {
    background: var(--accent-soft-hover);
    color: var(--accent);
  }

  .primary,
  .primary:hover:not(:disabled) {
    color: white;
  }

  .primary {
    background: var(--accent-strong);
  }

  .primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  .danger {
    background: var(--bad-soft);
    color: var(--bad);
  }

  .danger:hover:not(:disabled) {
    background: var(--bad-soft-hover);
    color: var(--bad);
  }

  /* Ghost buttons take the colour of what they sit on (--ghost), solid so fades stay even. */
  .ghost {
    background: var(--ghost, var(--surface));
  }

  .ghost:hover:not(:disabled) {
    background: var(--raised-hover);
  }
</style>
