<script lang="ts">
  let {
    value = 0,
    indeterminate = false,
    color = "var(--accent)",
  }: { value?: number; indeterminate?: boolean; color?: string } = $props();
</script>

<div
  class="track"
  role="progressbar"
  aria-valuenow={indeterminate ? undefined : Math.round(value * 100)}
>
  {#if indeterminate}
    <div class="sweep" style:background={color}></div>
  {:else}
    <div class="fill" style:width="{Math.min(1, value) * 100}%" style:background={color}></div>
  {/if}
</div>

<style>
  .track {
    height: 4px;
    overflow: hidden;
    border-radius: 2px;
    background: var(--track);
  }

  .fill {
    height: 100%;
    min-width: 4px;
    border-radius: 2px;
    transition:
      width 400ms cubic-bezier(0.33, 1, 0.68, 1),
      background-color var(--medium);
  }

  .sweep {
    width: 30%;
    height: 100%;
    border-radius: 2px;
    animation: sweep 1.3s ease-in-out infinite;
  }

  @keyframes sweep {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(340%);
    }
  }
</style>
