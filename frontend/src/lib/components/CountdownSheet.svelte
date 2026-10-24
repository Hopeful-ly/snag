<script lang="ts">
  import IconButton from "./IconButton.svelte";
  import Sheet from "./Sheet.svelte";
  import { countdownAction } from "../queues";
  import { useSnag } from "../state/context";

  // Closing the sheet also cancels, so the safe choice is the easy one.
  const snag = useSnag();
  const c = $derived(snag.countdown);
  let now = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 250);
    return () => clearInterval(timer);
  });
  const total = 60_000;
  const left = $derived(c ? Math.max(0, new Date(c.at).getTime() - now) : 0);
  const seconds = $derived(Math.ceil(left / 1000));
  const words = $derived(c ? countdownAction(c) : null);
</script>

{#if c && words}
  <Sheet title="{c.name} is done" icon="power" width={440} onclose={() => snag.cancelCountdown()}>
    <p role="alert">
      Snag will {words.will} in
      <strong class="numbers"
        >{Math.floor(seconds / 60)}:{String(seconds % 60).padStart(2, "0")}</strong
      >.
    </p>
    <div class="track" aria-hidden="true">
      <div class="fill" style:width="{Math.min(100, (left / total) * 100)}%"></div>
    </div>
    <div class="buttons">
      <IconButton
        icon="x"
        label={words.keep}
        kind="primary"
        busy={!!snag.busy["countdown:cancel"]}
        onclick={() => snag.cancelCountdown()}
      />
      <IconButton
        icon="power"
        label={words.now}
        kind="ghost"
        busy={!!snag.busy["countdown:now"]}
        onclick={() => snag.actNow()}
      />
    </div>
  </Sheet>
{/if}

<style>
  p {
    margin: 0 0 12px;
    color: var(--text-dim);
    font-size: var(--text-md);
  }

  strong {
    color: var(--text);
    font-weight: 600;
  }

  .track {
    height: 4px;
    border-radius: 2px;
    background: var(--track);
  }

  .fill {
    height: 4px;
    border-radius: 2px;
    background: var(--warn);
    transition: width 250ms linear;
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 18px;
  }
</style>
