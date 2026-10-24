<script lang="ts">
  import IconButton from "./IconButton.svelte";
  import Spinner from "./Spinner.svelte";

  let {
    value,
    min,
    max,
    busy = false,
    label,
    onchange,
  }: {
    value: number;
    min: number;
    max: number;
    busy?: boolean;
    label: string;
    onchange: (value: number) => void;
  } = $props();
</script>

<div class="stepper" role="group" aria-label={label}>
  <IconButton
    icon="minus"
    tip="Fewer"
    size={30}
    iconSize={14}
    disabled={value <= min || busy}
    onclick={() => onchange(value - 1)}
  />
  <span class="value numbers" aria-live="polite">
    {#if busy}<Spinner size={14} />{:else}{value}{/if}
  </span>
  <IconButton
    icon="plus"
    tip="More"
    size={30}
    iconSize={14}
    disabled={value >= max || busy}
    onclick={() => onchange(value + 1)}
  />
</div>

<style>
  .stepper {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .value {
    display: grid;
    place-items: center;
    width: 34px;
    font-size: var(--text-md);
    font-weight: 600;
  }
</style>
