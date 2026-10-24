<script lang="ts">
  import IconButton from "./IconButton.svelte";
  import Sheet from "./Sheet.svelte";
  import type { SubscriptionChange } from "../api/client";
  import { everyText, host, subscriptionStatus } from "../format";
  import { changeOf } from "../state/snag.svelte";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const s = $derived(snag.subscriptions.find((x) => x.id === snag.subscriptionSheet));
  const everyChoices = [1, 6, 12, 24, 168];

  let form = $state<SubscriptionChange | null>(null);
  let formFor = "";
  let picking = $state(false);
  $effect(() => {
    if (s && formFor !== s.id) {
      formFor = s.id;
      form = changeOf(s);
    }
  });

  const saving = $derived(!!s && !!snag.busy[`subscription:${s.id}:save`]);
  const youTube = $derived(!!s && /youtube\.com|youtu\.be/.test(s.source));
  const status = $derived(s ? subscriptionStatus(s) : "");

  function close() {
    snag.subscriptionSheet = null;
  }

  async function save() {
    const target = s;
    const change = form;
    if (!target || !change) return;
    const ok = await snag.request(`subscription:${target.id}:save`, () =>
      snag.api.changeSubscription(target.id, $state.snapshot(change)),
    );
    if (ok) close();
  }

  async function pickFolder() {
    if (!form) return;
    picking = true;
    try {
      const chosen = await snag.api.chooseFolder(form.directory);
      if (chosen && form) form.directory = chosen;
    } catch {
      // Closing the folder dialog is no error.
    } finally {
      picking = false;
    }
  }
</script>

{#if s && form}
  <Sheet title={s.title} icon="subscribe" width={560} locked={saving} onclose={close}>
    <p class="lead">
      <span class="source" title={s.source}>{host(s.source) || s.source}</span>
      <span class="status" class:bad={!!s.error}>{status}</span>
    </p>
    {#if s.error}
      <p class="bad" role="alert">Snag couldn't read it lately: {s.error}</p>
    {/if}
    {#if snag.startAtLogin?.available && !snag.startAtLogin.on && !form.paused}
      <p class="note">
        Snag checks for new ones only while it's running. Turn on starting at login so it always is.
        <button type="button" class="inline" onclick={() => snag.setStartAtLogin(true)}
          >Turn it on</button
        >
      </p>
    {/if}

    <label class="caption" for="subscription-title">Name</label>
    <input id="subscription-title" class="field" bind:value={form.title} disabled={saving} />

    <div class="caption">Look for new ones</div>
    <div class="chips" role="radiogroup" aria-label="Look for new ones">
      {#each everyChoices as hours (hours)}
        <button
          role="radio"
          aria-checked={form.everyHours === hours}
          class:chosen={form.everyHours === hours}
          disabled={saving}
          onclick={() => form && (form.everyHours = hours)}>{everyText(hours)}</button
        >
      {/each}
    </div>

    <div class="caption">Quality</div>
    <div class="chips" role="radiogroup" aria-label="Quality">
      {#each s.qualities as q (q.id)}
        <button
          role="radio"
          aria-checked={form.quality === q.id}
          class:chosen={form.quality === q.id}
          disabled={saving}
          onclick={() => form && (form.quality = q.id)}>{q.label}</button
        >
      {/each}
    </div>

    <label class="caption" for="subscription-folder">Save to</label>
    <div class="row">
      <input id="subscription-folder" class="field" bind:value={form.directory} disabled={saving} />
      <IconButton
        icon="folder"
        tip="Choose a folder…"
        size={38}
        busy={picking}
        disabled={saving}
        onclick={pickFolder}
      />
    </div>

    <div class="caption">Leave out</div>
    <div class="row">
      <input
        class="field"
        aria-label="Only titles with"
        placeholder="Only titles with (commas between)"
        bind:value={form.filters.include}
        disabled={saving}
      />
      <input
        class="field"
        aria-label="No titles with"
        placeholder="No titles with (trailer, teaser)"
        bind:value={form.filters.exclude}
        disabled={saving}
      />
    </div>
    <div class="row checks">
      {#if youTube}
        <label class="check"
          ><input type="checkbox" bind:checked={form.filters.noShorts} disabled={saving} />No Shorts</label
        >
      {/if}
      <label class="check">
        Shorter than
        <input
          class="minutes"
          type="number"
          min="0"
          max="600"
          aria-label="Shorter than, in minutes"
          bind:value={form.filters.minMinutes}
          disabled={saving}
        />
        minutes
      </label>
      <label class="check"
        ><input type="checkbox" bind:checked={form.paused} disabled={saving} />Paused</label
      >
    </div>

    <div class="buttons">
      <IconButton
        icon="trash"
        label="Unsubscribe"
        kind="danger"
        tip="Stop following it; what it downloaded stays"
        busy={!!snag.busy[`subscription:${s.id}:remove`]}
        disabled={saving}
        onclick={() => snag.actOnSubscription(s.id, "remove")}
      />
      <span class="spacer"></span>
      <IconButton
        icon="retry"
        label="Check now"
        busy={s.checking || !!snag.busy[`subscription:${s.id}:check`]}
        disabled={saving || s.checking}
        onclick={() => snag.actOnSubscription(s.id, "check")}
      />
      <IconButton icon="check" label="Save" kind="primary" busy={saving} onclick={save} />
    </div>
  </Sheet>
{/if}

<style>
  .lead {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
    margin: 0 0 6px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .source {
    color: var(--text);
  }

  .bad {
    color: var(--bad);
    font-size: var(--text-sm);
  }

  .note {
    margin: 8px 0 0;
    color: var(--warn);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .inline {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
    cursor: pointer;
  }

  .caption {
    display: block;
    margin: 14px 0 6px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .field {
    box-sizing: border-box;
    flex: 1;
    width: 100%;
    min-width: 0;
    height: 38px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 9px;
    outline: 0;
    background: var(--field);
    color: var(--text);
    font-size: var(--text-md);
    color-scheme: dark;
  }

  .field:focus {
    border-color: var(--accent-strong);
  }

  .row {
    display: flex;
    gap: 8px;
  }

  .checks {
    flex-wrap: wrap;
    gap: 8px 18px;
    margin-top: 10px;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .check input[type="checkbox"] {
    margin: 0;
  }

  .minutes {
    width: 56px;
    height: 28px;
    padding: 0 6px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--field);
    color: var(--text);
    color-scheme: dark;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .chips button {
    height: 28px;
    padding: 0 11px;
    border: 1px solid var(--raised);
    border-radius: 14px;
    background: var(--raised);
    color: var(--text-dim);
    font-size: var(--text-sm);
    cursor: pointer;
  }

  .chips button.chosen {
    border-color: var(--accent-strong);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .buttons {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 20px;
  }

  .spacer {
    flex: 1;
  }
</style>
