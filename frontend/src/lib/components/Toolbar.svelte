<script lang="ts">
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import { size, subscriptionLine } from "../format";
  import { inQueue, mainQueue, scheduleText } from "../queues";
  import { useSnag } from "../state/context";

  let { search = $bindable() }: { search?: HTMLInputElement } = $props();

  const snag = useSnag();
  const list = $derived(snag.downloads.list);
  const shown = $derived(snag.visible.length);
  const canResume = $derived(list.some((d) => d.status === "paused" || d.status === "error"));
  const canPause = $derived(
    list.some((d) => ["downloading", "queued", "pausing"].includes(d.status)),
  );
  const pausing = $derived(list.some((d) => d.status === "pausing"));
  const queue = $derived(snag.queue);
  const subscription = $derived(snag.subscription);
  const queueBytes = $derived(
    snag.visible.reduce((sum, d) => sum + Math.max(0, (d.size || 0) - (d.done || 0)), 0),
  );
  const summary = $derived(
    !snag.ready
      ? "Starting…"
      : `${shown} ${shown === 1 ? "download" : "downloads"}` +
          (snag.search ? ` matching “${snag.search}”` : "") +
          (queue && queueBytes ? `  ·  ${size(queueBytes)} to go` : "") +
          (queue
            ? queue.running
              ? "  ·  running"
              : scheduleText(queue)
                ? "  ·  waiting for its time"
                : "  ·  stopped"
            : subscription
              ? `  ·  ${subscriptionLine(subscription)}`
              : ""),
  );
  const runBusy = $derived(queue ? !!snag.busy[`queue:${queue.id}:run`] : false);
</script>

<header>
  <div class="where">
    <h1>{snag.title}</h1>
    <div class="count">{summary}</div>
  </div>

  <div class="tools">
    {#if queue}
      {#if queue.running}
        <IconButton
          icon="stop"
          label="Stop"
          tip={queue.id === mainQueue
            ? "Stop Main: its downloads wait until you start it"
            : "Stop: its downloads wait in line"}
          busy={runBusy}
          onclick={() => snag.stopQueue(queue.id)}
        />
      {:else}
        <IconButton
          icon="play"
          label="Start now"
          kind="primary"
          tip="Run this queue now"
          busy={runBusy}
          onclick={() => snag.startQueue(queue.id)}
        />
      {/if}
      <IconButton
        icon="reverse"
        label="Reverse"
        tip="Put the last download first"
        disabled={snag.downloads.list.filter((d) => inQueue(d, queue.id)).length < 2}
        busy={!!snag.busy[`queue:${queue.id}:reverse`]}
        onclick={() => snag.reverseQueue(queue.id)}
      />
      <IconButton
        icon="queue"
        label="Edit"
        tip="Edit queue"
        onclick={() => (snag.queueSheet = { queue })}
      />
      {#if queue.id !== mainQueue}
        <IconButton
          icon="trash"
          tip="Delete queue…"
          onclick={() => (snag.queueSheet = { queue, deleting: true })}
        />
      {/if}
      <span class="divider"></span>
    {:else if subscription}
      <IconButton
        icon="retry"
        label="Check now"
        tip="Look for what's new in it now"
        busy={subscription.checking || !!snag.busy[`subscription:${subscription.id}:check`]}
        disabled={subscription.checking}
        onclick={() => snag.actOnSubscription(subscription.id, "check")}
      />
      <IconButton
        icon="settings"
        label="Edit"
        tip="Edit subscription"
        onclick={() => snag.actOnSubscription(subscription.id, "edit")}
      />
      <span class="divider"></span>
    {/if}
    <label class="search" class:wide={snag.search}>
      <Icon name="search" size={15} />
      <input
        bind:this={search}
        bind:value={snag.search}
        placeholder="Search (Ctrl+F)"
        aria-label="Search"
        onkeydown={(e) => {
          if (e.key === "Escape") {
            snag.search = "";
            search?.blur();
            e.stopPropagation();
          }
        }}
      />
      {#if snag.search}
        <IconButton
          icon="x"
          tip="Clear search"
          kind="ghost"
          size={26}
          iconSize={13}
          onclick={() => (snag.search = "")}
        />
      {/if}
    </label>

    <IconButton
      icon="plus"
      kind="primary"
      active={snag.addOpen}
      tip={snag.addOpen ? "Close (Esc)" : "Add a link (Ctrl+V)"}
      onclick={() => (snag.addOpen = !snag.addOpen)}
    />
    <IconButton
      icon="play"
      tip="Resume all"
      busy={!!snag.busy["global:resumeAll"]}
      disabled={!canResume}
      onclick={() => snag.resumeAll()}
    />
    <IconButton
      icon="pause"
      tip="Pause all"
      busy={!!snag.busy["global:pauseAll"] || pausing}
      disabled={!canPause}
      onclick={() => snag.pauseAll()}
    />
    <IconButton
      icon="clearDone"
      tip="Clear finished from the list (keeps the files)"
      busy={!!snag.busy["global:clearDone"]}
      disabled={!snag.downloads.counts.finished}
      onclick={() => snag.clearDone()}
    />
    <IconButton
      icon="settings"
      tip="Settings"
      active={snag.settingsOpen}
      onclick={() => {
        snag.subscriptionsOpen = false;
        snag.settingsOpen = true;
      }}
    />
  </div>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    height: 66px;
    padding: 0 20px 0 22px;
  }

  .where {
    min-width: 0;
  }

  h1 {
    margin: 0;
    font-size: var(--text-xl);
    font-weight: 600;
  }

  .count {
    margin-top: 2px;
    overflow: hidden;
    color: var(--text-faint);
    font-size: var(--text-sm);
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .tools {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .divider {
    width: 1px;
    height: 22px;
    margin: 0 4px;
    background: var(--border);
  }

  .search {
    --ghost: var(--field);
    display: flex;
    align-items: center;
    gap: 8px;
    width: 190px;
    height: 34px;
    margin-right: 2px;
    padding: 0 4px 0 10px;
    border: 1px solid var(--border);
    border-radius: 9px;
    background: var(--field);
    color: var(--text-faint);
    transition:
      width var(--medium) cubic-bezier(0.33, 1, 0.68, 1),
      border-color var(--fast);
  }

  .search:focus-within,
  .search.wide {
    width: 240px;
  }

  .search:focus-within {
    border-color: var(--accent-strong);
  }

  input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: 0;
    background: none;
  }

  input::placeholder {
    color: var(--text-faint);
  }
</style>
