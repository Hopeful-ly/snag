<script lang="ts">
  import { fly } from "svelte/transition";
  import Icon from "./Icon.svelte";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const icon = { info: "check", good: "done", bad: "failed" } as const;
</script>

{#if snag.toast}
  {#key snag.toast.key}
    <div
      class="toast {snag.toast.kind}"
      role={snag.toast.kind === "bad" ? "alert" : "status"}
      transition:fly={{ y: 12, duration: 200 }}
    >
      <Icon name={icon[snag.toast.kind]} size={16} />
      <span>{snag.toast.text}</span>
    </div>
  {/key}
{/if}

<style>
  .toast {
    position: fixed;
    bottom: 18px;
    left: calc(50% + 110px);
    z-index: 900;
    display: flex;
    align-items: center;
    gap: 9px;
    max-width: min(560px, calc(100vw - 280px));
    height: 38px;
    padding: 0 14px;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--toast);
    transform: translateX(-50%);
  }

  span {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .info :global(svg) {
    color: var(--accent);
  }

  .good :global(svg) {
    color: var(--good);
  }

  .bad {
    border-color: #5a2a2a;
  }

  .bad :global(svg) {
    color: var(--bad);
  }
</style>
