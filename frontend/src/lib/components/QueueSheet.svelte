<script lang="ts">
  import { untrack } from "svelte";
  import IconButton from "./IconButton.svelte";
  import SettingRow from "./SettingRow.svelte";
  import Sheet from "./Sheet.svelte";
  import Stepper from "./Stepper.svelte";
  import Toggle from "./Toggle.svelte";
  import type { Queue } from "../api/client";
  import {
    blankQueue,
    clock,
    mainQueue,
    minutesOf,
    startsIn,
    weekdays,
    whenDoneChoices,
  } from "../queues";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const editing = untrack(() => snag.queueSheet?.queue ?? null);
  const isMain = editing?.id === mainQueue;

  const from: Queue = editing ? structuredClone($state.snapshot(editing)) : blankQueue();
  let name = $state(from.name);
  let atOnce = $state(from.atOnce || 2);
  let scheduled = $state(!!from.schedule?.on);
  let start = $state(from.schedule?.start || "02:00");
  let stop = $state(from.schedule?.stop || "");
  let days = $state<number[]>(
    from.schedule?.days?.length ? [...from.schedule.days] : [0, 1, 2, 3, 4, 5, 6],
  );
  const kib = 1024;
  const mib = 1024 * 1024;
  let limited = $state(!!from.speedLimit);
  let unit = $state(from.speedLimit && from.speedLimit >= mib ? mib : kib);
  let amount = $state(
    from.speedLimit
      ? Math.round((from.speedLimit / (from.speedLimit >= mib ? mib : kib)) * 10) / 10
      : 500,
  );
  let whenDone = $state(from.whenDone ?? "");
  let naming = $state(from.naming ?? "");
  let retries = $state(from.retries ?? 3);
  let retryWait = $state(from.retryWait ?? 5);
  let confirmDelete = $state(untrack(() => !!snag.queueSheet?.deleting));
  let field: HTMLInputElement | undefined = $state();

  $effect(() => {
    if (!editing) field?.focus();
  });

  const saving = $derived(!!snag.busy["queue:save"] || !!snag.busy["queue:delete"]);
  const valid = $derived(
    name.trim() !== "" && (!scheduled || (minutesOf(start) !== null && days.length > 0)),
  );
  const preview = $derived.by(() => {
    if (!scheduled || minutesOf(start) === null) return "";
    const sample = { ...blankQueue(), schedule: { on: true, start, stop, days } };
    const when = startsIn(sample);
    const first = `Starts at ${clock(start)}${when ? `, ${when}` : ""}.`;
    return stop
      ? `${first} Whatever is left at ${clock(stop)} waits until next time.`
      : `${first} It runs until everything in it is downloaded.`;
  });

  function toggleDay(day: number) {
    days = days.includes(day) ? days.filter((d) => d !== day) : [...days, day];
  }

  function clamp(n: number, lo: number, hi: number, fallback: number): number {
    if (!Number.isFinite(n)) return fallback;
    return Math.max(lo, Math.min(hi, Math.round(n)));
  }

  async function save() {
    if (!valid || saving) return;
    const q: Queue = {
      ...from,
      name: isMain ? from.name : name.trim(),
      atOnce,
      schedule: { on: scheduled, start, stop, days: days.length === 7 ? [] : [...days].sort() },
      speedLimit: limited ? Math.max(1, Math.round(amount * unit)) : 0,
      whenDone,
      naming: naming.trim(),
      retries: clamp(retries, 0, 20, 3),
      retryWait: clamp(retryWait, 0, 300, 5),
    };
    if (await snag.saveQueue(q)) snag.queueSheet = null;
  }

  const unfinished = editing ? snag.unfinishedIn(editing.id) : 0;
  async function remove(downloads: boolean) {
    if (!editing) return;
    if (await snag.deleteQueue(editing.id, downloads)) snag.queueSheet = null;
  }
</script>

<Sheet
  title={editing ? `Edit ${editing.name}` : "New queue"}
  icon="queue"
  width={540}
  locked={saving}
  onclose={() => {
    snag.queueSheet = null;
    snag.movingToNew = [];
  }}
>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      void save();
    }}
  >
    {#if !isMain}
      <label class="caption" for="queue-name">Name</label>
      <input
        id="queue-name"
        class="field"
        bind:this={field}
        bind:value={name}
        placeholder="Tonight, Big games…"
        maxlength="40"
        disabled={saving}
      />
    {/if}

    <label class="caption" for="queue-naming">Name playlist files</label>
    <input
      id="queue-naming"
      class="field"
      bind:value={naming}
      placeholder={"Same as in Settings, such as {artist} - {title}"}
      spellcheck="false"
      disabled={saving}
    />
    <p class="faint note">Empty fields drop out. Leave blank to use Settings.</p>

    <SettingRow title="Downloads at once" detail="The rest wait their turn, in order">
      <Stepper
        value={atOnce}
        min={1}
        max={10}
        label="Downloads at once"
        onchange={(v) => (atOnce = v)}
      />
    </SettingRow>

    <section class="schedule" aria-label="Schedule">
      <SettingRow
        title="Run on a schedule"
        detail={isMain ? "Off: Main runs all the time" : "Off: it runs when you press Start"}
      >
        <Toggle checked={scheduled} label="Run on a schedule" onchange={(v) => (scheduled = v)} />
      </SettingRow>
      {#if scheduled}
        <div class="times">
          <label>
            <span class="caption">Start at</span>
            <input class="field" type="time" bind:value={start} required disabled={saving} />
          </label>
          <label>
            <span class="caption">Pause at <span class="faint">(optional)</span></span>
            <input class="field" type="time" bind:value={stop} disabled={saving} />
          </label>
        </div>
        <div class="days" role="group" aria-label="Days">
          {#each weekdays as w (w.day)}
            <button
              type="button"
              aria-pressed={days.includes(w.day)}
              class:on={days.includes(w.day)}
              onclick={() => toggleDay(w.day)}>{w.short}</button
            >
          {/each}
        </div>
        <p class="preview" aria-live="polite">
          {days.length ? preview : "Pick at least one day."}
        </p>
      {/if}
    </section>

    <SettingRow
      title="Retries"
      detail="A failed download is tried again this many times, then the next one starts"
    >
      <Stepper value={retries} min={0} max={20} label="Retries" onchange={(v) => (retries = v)} />
    </SettingRow>
    <SettingRow
      title="Seconds between tries"
      detail="How long it waits before trying that one again"
    >
      <input
        class="field amount"
        type="number"
        min="0"
        max="300"
        step="1"
        aria-label="Seconds between tries"
        bind:value={retryWait}
        disabled={saving || retries === 0}
      />
    </SettingRow>

    <SettingRow title="Speed limit" detail="For this queue's downloads together">
      <div class="speed">
        {#if limited}
          <input
            class="field amount"
            type="number"
            min="1"
            step="any"
            aria-label="Speed limit"
            bind:value={amount}
            disabled={saving}
          />
          <select class="field unit" aria-label="Unit" bind:value={unit} disabled={saving}>
            <option value={kib}>KB/s</option>
            <option value={mib}>MB/s</option>
          </select>
        {/if}
        <Toggle checked={limited} label="Limit the speed" onchange={(v) => (limited = v)} />
      </div>
    </SettingRow>

    <div class="caption">When everything is downloaded</div>
    <div class="choices" role="radiogroup" aria-label="When everything is downloaded">
      {#each whenDoneChoices as c (c.id)}
        <button
          type="button"
          role="radio"
          aria-checked={whenDone === c.id}
          class:on={whenDone === c.id}
          onclick={() => (whenDone = c.id)}>{c.label}</button
        >
      {/each}
    </div>
    {#if whenDone}<p class="faint note">You get a minute to call it off first.</p>{/if}

    {#if editing && !isMain && confirmDelete}
      <div class="question" role="alertdialog" aria-label="Delete {editing.name}?">
        <p>
          Delete “{editing.name}”?
          {#if unfinished}
            {unfinished === 1 ? "Its download isn't" : `Its ${unfinished} downloads aren't`} finished:
            delete {unfinished === 1 ? "it" : "them"} too, or keep {unfinished === 1
              ? "it"
              : "them"} in Main. Finished files stay.
          {/if}
        </p>
        <div class="buttons">
          <IconButton
            icon="x"
            label="Back"
            kind="ghost"
            disabled={saving}
            onclick={() => (confirmDelete = false)}
          />
          <span class="spacer"></span>
          {#if unfinished}
            <IconButton
              icon="queue"
              label="Keep them in Main"
              tip="Delete only the queue; its downloads move to Main"
              disabled={saving}
              onclick={() => remove(false)}
            />
            <IconButton
              icon="trash"
              label="Delete them too"
              kind="danger"
              tip="Stops them and deletes what they downloaded so far"
              busy={!!snag.busy["queue:delete"]}
              disabled={saving}
              onclick={() => remove(true)}
            />
          {:else}
            <IconButton
              icon="trash"
              label="Delete queue"
              kind="danger"
              busy={!!snag.busy["queue:delete"]}
              disabled={saving}
              onclick={() => remove(false)}
            />
          {/if}
        </div>
      </div>
    {:else}
      <div class="buttons">
        {#if editing && !isMain}
          <IconButton
            icon="trash"
            label="Delete"
            kind="ghost"
            disabled={saving}
            onclick={() => (confirmDelete = true)}
          />
        {/if}
        <span class="spacer"></span>
        <IconButton
          icon="x"
          label="Cancel"
          kind="ghost"
          disabled={saving}
          onclick={() => {
            snag.queueSheet = null;
            snag.movingToNew = [];
          }}
        />
        <IconButton
          icon="check"
          label={editing ? "Save" : "Create queue"}
          kind="primary"
          busy={!!snag.busy["queue:save"]}
          disabled={!valid || saving}
          onclick={save}
        />
      </div>
    {/if}
  </form>
</Sheet>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .caption {
    display: block;
    margin: 6px 0 6px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .faint {
    color: var(--text-faint);
  }

  .field {
    box-sizing: border-box;
    width: 100%;
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

  .schedule {
    margin: 4px 0;
    padding: 4px 12px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--sidebar);
  }

  .times {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .days,
  .choices {
    display: flex;
    gap: 6px;
    margin-top: 10px;
  }

  .days button,
  .choices button {
    flex: 1;
    height: 32px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--field);
    color: var(--text-dim);
    cursor: pointer;
    transition: background-color var(--fast);
  }

  .days button.on {
    border-color: var(--accent-strong);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .choices {
    margin-top: 0;
    padding: 4px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--field);
  }

  .choices button {
    border: 0;
    background: var(--field);
  }

  .choices button.on {
    background: var(--surface-active);
    color: var(--text);
    font-weight: 600;
  }

  .preview,
  .note {
    margin: 8px 0 0;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .note {
    color: var(--text-faint);
  }

  .speed {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .amount {
    width: 90px;
  }

  .unit {
    width: 84px;
    padding: 0 8px;
  }

  .buttons {
    display: flex;
    gap: 8px;
    margin-top: 14px;
  }

  .spacer {
    flex: 1;
  }

  .question {
    margin-top: 14px;
    padding: 12px 14px;
    border: 1px solid var(--bad-soft-hover);
    border-radius: var(--radius-lg);
    background: var(--bad-soft);
  }

  .question p {
    margin: 0;
    font-size: var(--text-base);
    line-height: 1.45;
  }

  .question .buttons {
    flex-wrap: wrap;
    justify-content: flex-end;
    margin-top: 12px;
  }
</style>
