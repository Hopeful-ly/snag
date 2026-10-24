<script lang="ts">
  import Spinner from "./Spinner.svelte";

  let {
    checked,
    busy = false,
    label,
    onchange,
  }: {
    checked: boolean;
    busy?: boolean;
    label: string;
    onchange: (value: boolean) => void;
  } = $props();
</script>

<button
  type="button"
  role="switch"
  aria-checked={checked}
  aria-label={label}
  aria-busy={busy}
  class:on={checked}
  disabled={busy}
  onclick={() => onchange(!checked)}
>
  <span class="knob"
    >{#if busy}<Spinner size={10} />{/if}</span
  >
</button>

<style>
  button {
    position: relative;
    width: 40px;
    height: 22px;
    padding: 0;
    border: 0;
    border-radius: 11px;
    background: var(--track);
    cursor: pointer;
    transition: background-color var(--medium);
  }

  button.on {
    background: var(--accent-strong);
  }

  button:disabled {
    cursor: default;
  }

  .knob {
    position: absolute;
    top: 3px;
    left: 3px;
    display: grid;
    place-items: center;
    width: 16px;
    height: 16px;
    border-radius: 8px;
    background: white;
    color: var(--accent-strong);
    transition: transform var(--medium) cubic-bezier(0.33, 1, 0.68, 1);
  }

  .on .knob {
    transform: translateX(18px);
  }
</style>
