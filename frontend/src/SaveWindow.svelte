<script lang="ts">
  import { tick, untrack } from "svelte";
  import Icon from "./lib/components/Icon.svelte";
  import IconButton from "./lib/components/IconButton.svelte";
  import SettingRow from "./lib/components/SettingRow.svelte";
  import Toggle from "./lib/components/Toggle.svelte";
  import Tooltip from "./lib/components/Tooltip.svelte";
  import { tip } from "./lib/tooltip.svelte";
  import type { Api, Clash, Download, QueueState } from "./lib/api/client";
  import { clock, mainQueue, scheduleText } from "./lib/queues";
  import { categoryOf, extension, host, size } from "./lib/format";

  // Closing the window only defers the question; Snag closes it once answered.
  let { api, id }: { api: Api; id: string } = $props();
  const saving = untrack(() => ({ api, id })); // neither changes over the window's life

  type Answer = "download" | "later" | "now" | "cancel";
  type Settle = "replace" | "keep";

  let d = $state<Download | null>(null);
  let name = $state("");
  let suggested = "";
  let folder = $state("");
  let remember = $state(false);
  let places = $state<string[]>([]);
  let queues = $state<QueueState[]>([]);
  let queue = $state(mainQueue);
  const chosenQueue = $derived(queues.find((q) => q.id === queue));
  const waits = $derived(!!chosenQueue && chosenQueue.id !== mainQueue && !chosenQueue.running);
  let home = $state("");
  let busy = $state<Answer | "">("");
  let picking = $state(false);
  let failure = $state("");
  let field: HTMLInputElement | undefined = $state();
  let clash = $state<Clash | null>(null);
  let asking = $state<{ how: Exclude<Answer, "cancel">; clash: Clash } | null>(null);
  let settling = $state<Settle | "">("");
  let question: HTMLElement | undefined = $state();
  let page: HTMLElement | undefined = $state();
  // Kept so the taken-name question takes the same height and the buttons don't move.
  let formHeight = $state(0);

  const category = $derived(d ? categoryOf(d) : null);
  // yt-dlp adds the container's extension itself, so streams are named without one.
  const suffix = $derived(
    d?.kind !== "stream"
      ? ""
      : /mp3/i.test(d.label)
        ? ".mp3"
        : /audio|m4a/i.test(d.label)
          ? ".m4a"
          : ".mp4",
  );
  const kind = $derived(
    !d ? "" : d.label || (extension(d) ? `${extension(d).toUpperCase()} file` : "File"),
  );
  const from = $derived(
    !d
      ? ""
      : [d.size ? "~" + size(d.size) : "", host(d.page || d.url)].filter(Boolean).join("  ·  "),
  );

  async function load() {
    try {
      const [download, settings, folders, homeDir, queueList] = await Promise.all([
        saving.api.get(saving.id),
        saving.api.settings(),
        saving.api.places(),
        saving.api.home(),
        saving.api.queues().catch(() => []),
      ]);
      queues = queueList;
      d = download;
      suggested = download.name;
      name = download.name;
      folder = settings.downloadDir;
      places = folders;
      home = homeDir;
      queueMicrotask(() => {
        field?.focus();
        field?.select();
      });
    } catch (error) {
      failure = messageOf(error);
    }
  }
  void load();

  // The window follows the page's height. The empty page is only padding, so it isn't measured.
  // Fitting only once after the form opens would clip it.
  $effect(() => {
    const shown = page;
    if (!shown || (!d && !failure)) return;
    const fit = () => {
      const height = shown.getBoundingClientRect().height;
      if (!asking) formHeight = height;
      saving.api.fitWindow(height).catch(() => {});
    };
    const watch = new ResizeObserver(fit);
    watch.observe(shown);
    return () => watch.disconnect();
  });

  const typed = () => {
    const chosen = name.trim();
    return chosen ? chosen + suffix : "";
  };

  let checks = 0;
  $effect(() => {
    const dir = folder.trim();
    const file = typed();
    if (!d) return;
    const check = ++checks;
    const timer = setTimeout(async () => {
      const found = await saving.api.clash(saving.id, dir, file).catch(() => null);
      if (check === checks) clash = found;
    }, 200);
    return () => clearTimeout(timer);
  });

  async function send(how: Answer, settle?: Settle) {
    if (!d || busy) return;
    busy = how;
    settling = settle ?? "";
    failure = "";
    const chosen = name.trim();
    try {
      if (how === "cancel") {
        await saving.api.remove(d.id, false);
        return;
      }
      // Left as it came, no name is sent and Snag keeps the best one it knows.
      let filename = chosen && chosen !== suggested.trim() ? chosen + suffix : "";
      if (!settle) {
        // Asked afresh: the folder may have changed since the name was last looked at.
        const found = await saving.api.clash(saving.id, folder.trim(), typed());
        if (found) {
          await ask(how, found);
          return;
        }
      } else if (settle === "keep") {
        filename = asking!.clash.keepBoth + suffix;
      } else {
        filename = chosen + suffix;
      }
      await saving.api.confirm(d.id, {
        directory: folder.trim(),
        filename,
        start: how !== "later",
        remember,
        replace: settle === "replace",
        queue,
      });
      if (how === "now" && waits) await saving.api.startNow(d.id);
    } catch (error) {
      failure = messageOf(error);
      busy = settling = "";
    }
  }

  // Enter means Keep both: it loses nothing.
  async function ask(how: Exclude<Answer, "cancel">, found: Clash) {
    busy = "";
    clash = found;
    asking = { how, clash: found };
    await tick();
    question?.querySelector<HTMLButtonElement>('button[aria-label="Keep both"]')?.focus();
  }

  async function changeName() {
    asking = null;
    failure = "";
    await tick();
    field?.focus();
    field?.select();
  }

  async function pickFolder() {
    picking = true;
    try {
      const picked = await saving.api.chooseFolder(folder);
      if (picked) folder = picked;
    } catch {
      // Closing the folder dialog is no error.
    } finally {
      picking = false;
    }
  }

  const short = (path: string) =>
    home && path.startsWith(home) ? "~" + path.slice(home.length) : path;
  const onEnter = (e: KeyboardEvent) => e.key === "Enter" && send("download");

  function messageOf(error: unknown): string {
    return error instanceof Error ? error.message : String(error);
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key !== "Escape" || busy) return;
    if (asking) void changeName();
    else void saving.api.closeWindow();
  }}
/>

<main aria-label="Save download" bind:this={page}>
  {#if d && asking}
    {@const taken = asking.clash}
    <div
      class="question"
      tabindex="-1"
      role="alertdialog"
      aria-labelledby="question-title"
      aria-describedby="question-text"
      style:min-height={formHeight ? `${formHeight - 38}px` : undefined}
      bind:this={question}
    >
      <span class="warning"><Icon name="failed" size={22} /></span>
      <h2 id="question-title">
        {taken.inUse ? `“${taken.existing}” is taken` : `Replace “${taken.existing}”?`}
      </h2>
      <p id="question-text">
        {#if taken.inUse}
          Another download in Snag is saving to this name in {short(folder.trim())}.
        {:else}
          A file with this name is already in {short(folder.trim())}. Replacing it deletes that file
          once this download has finished.
        {/if}
      </p>
      <p class="keep">Keep both saves this one as “{taken.keepBoth + suffix}”.</p>

      {#if failure}<p class="failure" role="alert">{failure}</p>{/if}

      <div class="buttons">
        <IconButton
          icon="x"
          label="Change name"
          kind="ghost"
          size={38}
          disabled={busy !== ""}
          onclick={changeName}
        />
        <IconButton
          icon="save"
          label="Keep both"
          kind={taken.inUse ? "primary" : "plain"}
          size={38}
          tip={`Save it as “${taken.keepBoth + suffix}”`}
          busy={settling === "keep"}
          disabled={busy !== ""}
          onclick={() => send(asking!.how, "keep")}
        />
        {#if !taken.inUse}
          <IconButton
            icon="check"
            label="Replace"
            kind="primary"
            size={38}
            tip="Replace the file that's there"
            busy={settling === "replace"}
            disabled={busy !== ""}
            onclick={() => send(asking!.how, "replace")}
          />
        {/if}
      </div>
    </div>
  {:else if d && category}
    <div class="what">
      <span class="tile" style:background={category.tile} style:color={category.ink}>
        <Icon name={category.icon} size={19} />
      </span>
      <div>
        <div class="kind">{kind}</div>
        <div class="from">{from}</div>
      </div>
    </div>

    <label class="caption" for="save-name">Name</label>
    <div class="field">
      <input
        id="save-name"
        bind:this={field}
        bind:value={name}
        disabled={busy !== ""}
        onkeydown={onEnter}
      />
      {#if suffix}<span class="suffix">{suffix}</span>{/if}
      {#if clash}
        <span
          class="taken"
          role="img"
          aria-label={clash.inUse
            ? "Another download is saving to this name"
            : "Already in this folder"}
          use:tip={clash.inUse
            ? "Another download in Snag is saving to this name"
            : `“${clash.existing}” is already in this folder; you'll be asked before it's replaced`}
        >
          <Icon name="failed" size={16} />
        </span>
      {/if}
    </div>

    <label class="caption" for="save-folder">Save to</label>
    <div class="folder-row">
      <div class="field">
        <Icon name="folder" size={16} />
        <input id="save-folder" bind:value={folder} disabled={busy !== ""} onkeydown={onEnter} />
      </div>
      <IconButton
        icon="folder"
        tip="Choose a folder…"
        size={40}
        busy={picking}
        disabled={busy !== ""}
        onclick={pickFolder}
      />
    </div>
    <div class="places">
      {#each places as place (place)}
        <button
          class:chosen={folder === place}
          disabled={busy !== ""}
          onclick={() => (folder = place)}>{short(place)}</button
        >
      {/each}
    </div>

    {#if queues.length > 1}
      <div class="caption">Queue</div>
      <div class="places" role="radiogroup" aria-label="Queue">
        {#each queues as q (q.id)}
          <button
            role="radio"
            aria-checked={queue === q.id}
            class:chosen={queue === q.id}
            disabled={busy !== ""}
            onclick={() => (queue = q.id)}>{q.name}</button
          >
        {/each}
      </div>
      {#if waits && chosenQueue}
        <p class="queue-note">
          {scheduleText(chosenQueue)
            ? `${chosenQueue.name} starts at ${clock(chosenQueue.schedule.start)}; this one waits for it.`
            : `${chosenQueue.name} is stopped: this one waits until you start it.`}
        </p>
      {/if}
    {/if}

    <SettingRow
      title="Always save here"
      detail="Don't ask again; new downloads go straight to this folder"
    >
      <Toggle checked={remember} label="Always save here" onchange={(v) => (remember = v)} />
    </SettingRow>

    {#if failure}<p class="failure" role="alert">{failure}</p>{/if}

    <div class="buttons">
      <IconButton
        icon="x"
        label="Cancel"
        kind="ghost"
        size={38}
        tip="Don't download it"
        busy={busy === "cancel"}
        disabled={busy !== ""}
        onclick={() => send("cancel")}
      />
      {#if waits && chosenQueue}
        <IconButton
          icon="play"
          label="Download now"
          size={38}
          tip="Start it now instead of waiting"
          busy={busy === "now"}
          disabled={busy !== ""}
          onclick={() => send("now")}
        />
        <IconButton
          icon="clock"
          label="Add to {chosenQueue.name}"
          kind="primary"
          size={38}
          tip="It starts when {chosenQueue.name} runs (Enter)"
          busy={busy === "download"}
          disabled={busy !== ""}
          onclick={() => send("download")}
        />
      {:else}
        <IconButton
          icon="clock"
          label="Later"
          size={38}
          tip="Add it paused, start it yourself"
          busy={busy === "later"}
          disabled={busy !== ""}
          onclick={() => send("later")}
        />
        <IconButton
          icon="download"
          label="Download"
          kind="primary"
          size={38}
          tip="Start now (Enter)"
          busy={busy === "download"}
          disabled={busy !== ""}
          onclick={() => send("download")}
        />
      {/if}
    </div>
  {:else if failure}
    <p class="failure" role="alert">{failure}</p>
  {/if}
</main>
<Tooltip />

<style>
  :global(body) {
    background: var(--surface);
  }

  main {
    padding: 20px 22px 18px; /* 38px tall in all, which the question allows for */
  }

  .what {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .tile {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    border-radius: var(--radius-lg);
  }

  .kind {
    font-size: var(--text-md);
    font-weight: 600;
  }

  .from {
    margin-top: 3px;
    color: var(--text-dim);
    font-size: var(--text-sm);
    white-space: pre;
  }

  .caption {
    display: block;
    margin: 16px 0 6px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .field {
    display: flex;
    align-items: center;
    gap: 9px;
    flex: 1;
    height: 40px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 9px;
    background: var(--field);
    color: var(--text-faint);
    transition: border-color var(--fast);
  }

  .field:focus-within {
    border-color: var(--accent-strong);
  }

  input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: 0;
    background: none;
    font-size: var(--text-md);
  }

  .suffix {
    font-size: var(--text-md);
  }

  .taken {
    display: grid;
    color: var(--warn);
  }

  .question {
    display: flex;
    flex-direction: column;
  }

  .warning {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    border-radius: var(--radius-lg);
    background: var(--warn-soft);
    color: var(--warn);
  }

  .question h2 {
    margin: 16px 0 8px;
    font-size: var(--text-lg);
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .question p {
    margin: 0 0 8px;
    color: var(--text-dim);
    font-size: var(--text-md);
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .question .keep {
    font-size: var(--text-sm);
  }

  .question .buttons {
    margin-top: auto;
  }

  .folder-row {
    display: flex;
    gap: 8px;
  }

  .places {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 8px 0 6px;
  }

  .places button {
    height: 28px;
    padding: 0 11px;
    border: 1px solid var(--raised);
    border-radius: 14px;
    background: var(--raised);
    color: var(--text-dim);
    font-size: var(--text-sm);
    cursor: pointer;
    transition: background-color var(--fast);
  }

  .places button:hover:not(:disabled) {
    background: var(--raised-hover);
  }

  .places button.chosen {
    border-color: var(--accent-strong);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .queue-note {
    margin: 4px 0 2px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .failure {
    margin: 6px 0 0;
    color: var(--bad);
    font-size: var(--text-sm);
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 10px;
  }
</style>
