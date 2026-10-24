<script lang="ts">
  import { untrack } from "svelte";
  import Icon from "./lib/components/Icon.svelte";
  import IconButton from "./lib/components/IconButton.svelte";
  import TickList from "./lib/components/TickList.svelte";
  import Tooltip from "./lib/components/Tooltip.svelte";
  import type {
    Api,
    PlaylistEntry,
    PlaylistView,
    QueueState,
    StartAtLogin,
  } from "./lib/api/client";
  import { host } from "./lib/format";
  import { entryPreview, joinFolder, kindOf, templateOf } from "./lib/naming";
  import {
    clockOf,
    countOf,
    minutesOf,
    fromCurrent,
    groupsOf,
    lengthOf,
    nameOf,
    picked,
    startingPicks,
    toggleGroup,
  } from "./lib/playlist";

  let { api, id }: { api: Api; id: string } = $props();
  const listing = untrack(() => ({ api, id })); // neither changes over the window's life

  let view = $state<PlaylistView | null>(null);
  let picks = $state<boolean[]>([]);
  let have = $state<boolean[]>([]);
  let quality = $state("");
  let folder = $state("");
  let template = $state("");
  let edited = $state(false); // the template was typed, so changing queue or quality leaves it
  let nameField: HTMLInputElement | undefined = $state();
  let places = $state<string[]>([]);
  let home = $state("");
  // The queue they join: "" is a new one named after the list.
  let queues = $state<QueueState[]>([]);
  let queue = $state("");
  // The list shown bottom-first, so the last entry downloads first. Indexes sent stay the list's.
  let reversed = $state(false);
  let busy = $state(false);
  let picking = $state(false);
  let failure = $state("");
  let note = $state("");
  let follow = $state(false);
  let every = $state(6);
  let startAtLogin = $state<StartAtLogin | null>(null);
  const everyChoices = [
    { hours: 1, label: "Every hour" },
    { hours: 6, label: "Every 6 hours" },
    { hours: 24, label: "Every day" },
  ];

  const count = $derived(picks.filter(Boolean).length);
  const order = $derived.by(() => {
    const indexes = view ? [...view.entries.keys()] : [];
    if (reversed) indexes.reverse();
    return indexes;
  });
  const groups = $derived(view ? groupsOf(view.entries) : []);
  const audio = $derived(!!view?.qualities.find((q) => q.id === quality)?.audioOnly);
  const queueName = $derived(view?.title || "Playlist");
  const ownQueue = $derived(queues.find((q) => q.name.toLowerCase() === queueName.toLowerCase()));
  const about = $derived(
    view
      ? [host(view.page), countOf(view.entries.length, audio), lengthOf(view.entries)]
          .filter(Boolean)
          .join("  ·  ")
      : "",
  );
  function previewOf(entry: PlaylistEntry, i: number): string {
    if (!view) return "";
    return entryPreview(template, entry, i, view.entries.length, view.title);
  }

  async function load() {
    try {
      const [v, folders, homeDir, queueList] = await Promise.all([
        listing.api.playlist(listing.id),
        listing.api.places(),
        listing.api.home(),
        listing.api.queues().catch(() => []),
      ]);
      quality = v.quality;
      folder = v.folder;
      places = folders;
      home = homeDir;
      queues = queueList;
      queue =
        queueList.find((q) => q.name.toLowerCase() === (v.title || "Playlist").toLowerCase())?.id ??
        "";
      if (v.entries.length === 1) {
        // A song alone joins the rest of the downloads, not a folder and queue of its own.
        queue = "main";
      }
      have = await listing.api.playlistHave(listing.id, v.quality).catch(() => []);
      picks = startingPicks(v.entries.length, v.current, have);
      // Asked to follow it (the card's Subscribe): what is there is ticked only if picked.
      if (v.follow) picks = picks.map(() => false);
      follow = v.canFollow && (v.follow || !!v.following);
      every = v.following?.everyHours ?? 6;
      startAtLogin = await listing.api.startAtLogin().catch(() => null);
      view = v;
      template = templateOf(v, queueList, queue, v.quality);
    } catch (error) {
      failure = messageOf(error);
    }
  }
  void load();

  // Choosing a quality looks again at what Snag has: a video it has at 720p is new at 1080p.
  let looks = 0;
  async function chooseQuality(id: string) {
    quality = id;
    if (!edited && view) template = templateOf(view, queues, queue, id);
    const look = ++looks;
    const found = await listing.api.playlistHave(listing.id, id).catch(() => []);
    if (look === looks) have = found;
  }

  function chooseQueue(id: string) {
    queue = id;
    if (!edited && view) template = templateOf(view, queues, id, quality);
  }

  function insertField(name: string) {
    const token = `{${name}}`;
    const el = nameField;
    edited = true;
    if (!el) {
      template += token;
      return;
    }
    const start = el.selectionStart ?? template.length;
    const end = el.selectionEnd ?? start;
    template = template.slice(0, start) + token + template.slice(end);
    requestAnimationFrame(() => {
      el.focus();
      const at = start + token.length;
      el.setSelectionRange(at, at);
    });
  }

  async function download() {
    if (!view || busy || (!count && !follow)) return;
    busy = true;
    failure = "";
    try {
      const added = await listing.api.downloadPlaylist(listing.id, {
        entries: reversed ? [...picked(picks)].reverse() : picked(picks),
        quality,
        directory: folder.trim(),
        queue,
        naming: template.trim(),
        follow,
        everyHours: every,
      });
      note = added.note ?? "";
    } catch (error) {
      failure = messageOf(error);
    } finally {
      busy = false;
    }
  }

  async function pickFolder() {
    picking = true;
    try {
      const chosen = await listing.api.chooseFolder(folder);
      if (chosen) folder = chosen;
    } catch {
      // Closing the folder dialog is no error.
    } finally {
      picking = false;
    }
  }

  const short = (path: string) =>
    home && path.startsWith(home) ? "~" + path.slice(home.length) : path;

  function messageOf(error: unknown): string {
    return error instanceof Error ? error.message : String(error);
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Escape" && !busy) void listing.api.closeWindow();
  }}
/>

<main aria-label="Download playlist">
  {#if view}
    <div class="body">
      <div class="what">
        <span class="tile"><Icon name={audio ? "music" : "video"} size={19} /></span>
        <div class="texts">
          <h1>{view.title || "Playlist"}</h1>
          <div class="from">{about}</div>
        </div>
      </div>
      {#if view.note}
        <p class="how">{view.note}</p>
      {/if}

      <div class="choose">
        <div class="places">
          <button disabled={busy} onclick={() => (picks = picks.map(() => true))}>All</button>
          {#if view.current >= 0}
            <button
              disabled={busy}
              onclick={() => (picks = fromCurrent(picks.length, view!.current))}
              >From the playing one on</button
            >
          {/if}
          <button disabled={busy} onclick={() => (picks = picks.map(() => false))}>None</button>
          {#if view.entries.length > 1}
            <span class="divider"></span>
            <button
              aria-pressed={reversed}
              class:chosen={reversed}
              disabled={busy}
              title="Put the last one first"
              onclick={() => (reversed = !reversed)}>Reverse</button
            >
          {/if}
          {#if groups.length > 1}
            <span class="divider"></span>
            {#each groups as g (g.name)}
              {@const all = g.entries.every((i) => picks[i])}
              <button
                aria-pressed={all}
                class:chosen={all}
                class:some={!all && g.entries.some((i) => picks[i])}
                disabled={busy}
                onclick={() => (picks = toggleGroup(picks, g))}>{g.name}</button
              >
            {/each}
          {/if}
        </div>
        <span class="count">{count} of {picks.length}</span>
      </div>

      <TickList
        bind:picks
        {order}
        {busy}
        name="Entries"
        label={(i) => (view?.entries[i] ? nameOf(view.entries[i]) : "")}
      >
        {#snippet row(i: number)}
          {@const entry = view!.entries[i]!}
          {@const shown = nameOf(entry)}
          {@const preview = previewOf(entry, i)}
          <span class="number">{i + 1}</span>
          <span class="name">{preview || shown}</span>
          {#if preview && preview !== shown}<span class="was">{shown}</span>{/if}
          {#if i === view!.current}<span class="mark playing">Playing</span>{/if}
          {#if have[i]}<span class="mark">In Snag</span>{/if}
          <!-- A streaming site's episodes come from TMDB, which knows their minutes only. -->
          <span class="length"
            >{view!.via === "browser" ? minutesOf(entry.duration) : clockOf(entry.duration)}</span
          >
        {/snippet}
      </TickList>

      <div class="caption">Quality</div>
      <div class="places" role="radiogroup" aria-label="Quality">
        {#each view.qualities as q (q.id)}
          <button
            role="radio"
            aria-checked={quality === q.id}
            class:chosen={quality === q.id}
            disabled={busy}
            onclick={() => chooseQuality(q.id)}>{q.label}</button
          >
        {/each}
      </div>
      {#if view.via === "browser"}
        <!-- A streaming site's episodes come only as its player plays them. -->
        <p class="how">
          Each episode plays for a moment in a muted background tab of your browser, which finds its
          video; keep the browser open while they download. A site that sends one quality only gives
          that one.
        </p>
      {:else if view.via === "search"}
        <!-- Spotify's and Apple Music's own sound is protected: each song comes from YouTube Music. -->
        <p class="how">
          Each song is found on YouTube Music as it downloads, and saved with {view.site
            ? `${view.site}'s`
            : "its"} details and cover. If it isn't there, you're asked where else to look.
        </p>
      {/if}

      <label class="caption" for="playlist-folder">Save to</label>
      <div class="folder-row">
        <div class="field">
          <Icon name="folder" size={16} />
          <input id="playlist-folder" bind:value={folder} disabled={busy} />
        </div>
        <IconButton
          icon="folder"
          tip="Choose a folder…"
          size={40}
          busy={picking}
          disabled={busy}
          onclick={pickFolder}
        />
      </div>
      <div class="places">
        {#each places as place (place)}
          {@const there = joinFolder(place, view.folderName ?? "")}
          <button
            class:chosen={folder === place || folder === there}
            disabled={busy}
            onclick={() => (folder = there)}>{short(place)}</button
          >
        {/each}
      </div>

      <label class="caption" for="playlist-naming">Name files</label>
      <div class="folder-row">
        <div class="field">
          <input
            id="playlist-naming"
            bind:this={nameField}
            bind:value={template}
            disabled={busy}
            spellcheck="false"
            placeholder={"{artist} - {title}"}
            oninput={() => (edited = true)}
          />
        </div>
      </div>
      <div class="places" role="group" aria-label="Name fields">
        {#each view.fields?.[kindOf(view, quality)] ?? [] as f (f.name)}
          <button type="button" disabled={busy} title={f.about} onclick={() => insertField(f.name)}
            >{`{${f.name}}`}</button
          >
        {/each}
      </div>
      <div class="caption">Queue</div>
      <div class="places" role="radiogroup" aria-label="Queue">
        {#if !ownQueue}
          <button
            role="radio"
            aria-checked={queue === ""}
            class:chosen={queue === ""}
            disabled={busy}
            onclick={() => chooseQueue("")}>New: {queueName}</button
          >
        {/if}
        {#each queues as q (q.id)}
          <button
            role="radio"
            aria-checked={queue === q.id}
            class:chosen={queue === q.id}
            disabled={busy}
            onclick={() => chooseQueue(q.id)}>{q.name}</button
          >
        {/each}
      </div>

      {#if view.canFollow}
        <label class="follow">
          <input type="checkbox" bind:checked={follow} disabled={busy} />
          Keep downloading new ones
        </label>
        {#if follow}
          <div class="places" role="radiogroup" aria-label="Look for new ones">
            {#each everyChoices as c (c.hours)}
              <button
                role="radio"
                aria-checked={every === c.hours}
                class:chosen={every === c.hours}
                disabled={busy}
                onclick={() => (every = c.hours)}>{c.label}</button
              >
            {/each}
          </div>
          <p class="how">
            What is ticked downloads now. Everything else on the list counts as seen: only what
            comes after downloads, as it appears, the same way.{#if view.following}
              Snag follows this list already; this changes how.{/if}
          </p>
          {#if startAtLogin?.available && !startAtLogin.on}
            <p class="note login">
              Snag checks for new ones only while it's running. Turn on starting at login so it
              always is.
              <button
                type="button"
                class="inline"
                disabled={busy}
                onclick={async () => {
                  startAtLogin = await listing.api.setStartAtLogin(true).catch(() => startAtLogin);
                }}>Turn it on</button
              >
            </p>
          {/if}
        {:else if view.following}
          <p class="how">Snag follows this list. To stop, unsubscribe in Snag's sidebar.</p>
        {/if}
      {/if}

      {#if failure}<p class="failure" role="alert">{failure}</p>{/if}
      {#if note}<p class="note" role="status">{note}</p>{/if}
    </div>

    <div class="buttons">
      {#if note}
        <IconButton
          icon="check"
          label="Done"
          kind="primary"
          size={38}
          onclick={() => listing.api.closeWindow()}
        />
      {:else}
        <IconButton
          icon="x"
          label="Cancel"
          kind="ghost"
          size={38}
          disabled={busy}
          onclick={() => listing.api.closeWindow()}
        />
        <IconButton
          icon="download"
          label={follow
            ? count
              ? `Download ${count} and subscribe`
              : "Subscribe"
            : `Download ${count}`}
          kind="primary"
          size={38}
          tip={count
            ? `Add them to ${ownQueue?.name ?? queueName}`
            : follow
              ? "Download only what comes after"
              : "Tick some first"}
          {busy}
          disabled={busy || (!count && !follow)}
          onclick={download}
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
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 20px 22px 18px;
  }

  .body {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    overflow-y: auto;
  }

  .what {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .tile {
    display: grid;
    place-items: center;
    flex: none;
    width: 40px;
    height: 40px;
    border-radius: var(--radius-lg);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .texts {
    min-width: 0;
  }

  h1 {
    margin: 0;
    overflow: hidden;
    font-size: var(--text-md);
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .from {
    margin-top: 3px;
    color: var(--text-dim);
    font-size: var(--text-sm);
    white-space: pre;
  }

  .choose {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 12px;
  }

  .count {
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .places .divider {
    width: 1px;
    margin: 4px 2px;
    background: var(--border);
  }

  .places button.some {
    border-color: var(--accent-soft-hover);
    color: var(--text);
  }

  .number {
    flex: none;
    min-width: 2.2em;
    color: var(--text-faint);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    text-align: right;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .was {
    flex: 0 1 auto;
    max-width: 42%;
    min-width: 0;
    overflow: hidden;
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mark {
    flex: none;
    padding: 1px 7px;
    border-radius: 9px;
    background: var(--raised);
    color: var(--text-dim);
    font-size: var(--text-xs);
  }

  .mark.playing {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .length {
    flex: none;
    color: var(--text-faint);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  .caption {
    display: block;
    margin: 14px 0 6px;
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

  .follow {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 16px;
    font-size: var(--text-base);
    cursor: pointer;
  }

  .follow input {
    flex: none;
    margin: 0;
  }

  .login {
    margin-top: 10px;
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

  .how {
    margin: 14px 0 0;
    color: var(--text-dim);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .failure,
  .note {
    margin: 6px 0 0;
    font-size: var(--text-sm);
  }

  .failure {
    color: var(--bad);
  }

  .note {
    color: var(--warn);
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
