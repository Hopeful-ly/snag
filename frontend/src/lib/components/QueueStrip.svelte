<script lang="ts">
  import Icon from "./Icon.svelte";
  import { mainQueue, retryText, scheduleText, speedText, startsIn, whenDoneText } from "../queues";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const q = $derived(snag.queue);
  const schedule = $derived(q ? scheduleText(q) : "");
  const done = $derived(q ? whenDoneText(q.whenDone) : "");
</script>

{#if q}
  <section aria-label="How {q.name} runs">
    {#if schedule}
      <span class="chip accent" title={q.running ? "" : `Starts ${startsIn(q)}`}>
        <Icon name="clock" size={15} />{schedule}
      </span>
    {:else}
      <span class="chip">
        <Icon name="clock" size={15} />{q.id === mainQueue
          ? "Runs all the time"
          : "Runs when you start it"}
      </span>
    {/if}
    <span class="chip"><Icon name="queue" size={15} />{q.atOnce} at a time</span>
    <span class="chip"><Icon name="retry" size={15} />{retryText(q.retries, q.retryWait)}</span>
    <span class="chip"><Icon name="speed" size={15} />{speedText(q.speedLimit)}</span>
    {#if done}<span class="chip warn"><Icon name="power" size={15} />{done}</span>{/if}
    <button class="change" onclick={() => (snag.queueSheet = { queue: q })}>Change</button>
  </section>
{/if}

<style>
  section {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin: 0 16px 10px;
    padding: 9px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    height: 28px;
    padding: 0 11px;
    border-radius: 14px;
    background: var(--raised);
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .accent {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .warn {
    background: var(--warn-soft);
    color: var(--warn);
  }

  .change {
    margin-left: auto;
    padding: 4px 6px;
    border: 0;
    border-radius: 6px;
    background: var(--surface);
    color: var(--accent);
    font-size: var(--text-sm);
    cursor: pointer;
  }

  .change:hover {
    background: var(--surface-hover);
  }
</style>
