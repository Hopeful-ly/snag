<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    title,
    detail = "",
    children,
  }: { title: string; detail?: string; children?: Snippet } = $props();
</script>

<div class="row">
  <div class="texts">
    <div class="title">{title}</div>
    {#if detail}<div class="detail">{detail}</div>{/if}
  </div>
  {@render children?.()}
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-height: 52px;
    padding: var(--space-2) 0;
  }

  .row + :global(.row) {
    border-top: 1px solid var(--border);
  }

  .texts {
    flex: 1;
    min-width: 0;
  }

  .title {
    font-size: var(--text-md);
  }

  .detail {
    margin-top: 3px;
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
</style>
