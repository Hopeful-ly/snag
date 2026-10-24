<script lang="ts">
  import type { Snippet } from "svelte";
  import { fade, fly } from "svelte/transition";
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import type { IconName } from "../icons";

  let {
    title,
    icon,
    width = 520,
    locked = false,
    onclose,
    children,
  }: {
    title: string;
    icon: IconName;
    width?: number;
    locked?: boolean;
    onclose: () => void;
    children: Snippet;
  } = $props();

  const close = () => !locked && onclose();
  let card: HTMLElement | undefined = $state();

  // Take focus unless something inside already has it (the Save dialog's name field).
  $effect(() => {
    if (card && !card.contains(document.activeElement)) card.focus();
  });
</script>

<div class="scrim" role="presentation" transition:fade={{ duration: 200 }} onclick={close}>
  <div
    class="card"
    bind:this={card}
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    style:width="min({width}px, calc(100vw - 40px))"
    transition:fly={{ y: 12, duration: 200 }}
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => {
      if (e.key === "Escape") {
        close();
        e.stopPropagation();
      }
    }}
  >
    <header>
      <span class="icon"><Icon name={icon} size={19} /></span>
      <h2>{title}</h2>
      <IconButton
        icon="x"
        tip="Close (Esc)"
        kind="ghost"
        size={32}
        iconSize={16}
        disabled={locked}
        onclick={close}
      />
    </header>
    <div class="body">{@render children()}</div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 700;
    display: grid;
    place-items: center;
    background: var(--scrim);
  }

  .card {
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    background: var(--surface);
    outline: none;
  }

  header {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 58px;
    padding: 0 12px 0 20px;
  }

  .icon {
    display: grid;
    color: var(--accent);
  }

  h2 {
    flex: 1;
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .body {
    padding: 0 20px 18px;
  }
</style>
