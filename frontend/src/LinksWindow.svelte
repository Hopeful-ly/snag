<script lang="ts">
  import { untrack } from "svelte";
  import Icon from "./lib/components/Icon.svelte";
  import IconButton from "./lib/components/IconButton.svelte";
  import TickList from "./lib/components/TickList.svelte";
  import Tooltip from "./lib/components/Tooltip.svelte";
  import type { Api, Link, LinksView, QueueState } from "./lib/api/client";
  import { host, size } from "./lib/format";
  import { groupNames, isFile, matches, startingPicks } from "./lib/links";
  import { picked } from "./lib/playlist";

  let { api, id }: { api: Api; id: string } = $props();
  const listing = untrack(() => ({ api, id })); // neither changes over the window's life

  let view = $state<LinksView | null>(null);
  let picks = $state<boolean[]>([]);
  let have = $state<boolean[]>([]);
  let filter = $state("");
  let pages = $state(false); // show web pages, which few want
  let sameSite = $state(false);
  let hideHave = $state(false);
  let folder = $state("");
  let keepFolders = $state(true);
  let places = $state<string[]>([]);
  let home = $state("");
  let queues = $state<QueueState[]>([]);
  let queue = $state("main");
  let busy = $state(false);
  let picking = $state(false);
  let failure = $state("");

  const pageHost = $derived(view?.page ? host(view.page) : "");
  const order = $derived.by(() => {
    const list = view?.links ?? [];
    return [...list.keys()].filter((i) => {
      const l = list[i]!;
      if (!pages && l.group === "pages") return false;
      if (sameSite && pageHost && host(l.url) !== pageHost) return false;
      if (hideHave && have[i]) return false;
      return matches(l, filter);
    });
  });
  const chosen = $derived(order.filter((i) => picks[i]));
  const groups = $derived.by(() => {
    const list = view?.links ?? [];
    return Object.keys(groupNames)
      .map((g) => ({
        id: g,
        name: groupNames[g]!,
        entries: order.filter((i) => list[i]?.group === g),
      }))
      .filter((g) => g.entries.length > 0);
  });
  const hasPages = $derived(!!view?.links.some((l) => l.group === "pages"));
  const hasFolders = $derived(!!view?.links.some((l) => l.dir));
  const hasOtherSites = $derived(!!pageHost && !!view?.links.some((l) => host(l.url) !== pageHost));
  const total = $derived(chosen.reduce((sum, i) => sum + (view?.links[i]?.size ?? 0), 0));
  const about = $derived(
    view
      ? [pageHost || (view.source === "pasted" ? "Pasted" : ""), countOf(view.links.length)]
          .filter(Boolean)
          .join("  ·  ")
      : "",
  );

  function countOf(n: number): string {
    return n === 1 ? "1 link" : `${n} links`;
  }

  async function load() {
    try {
      const [v, folders, homeDir, queueList] = await Promise.all([
        listing.api.links(listing.id),
        listing.api.places(),
        listing.api.home(),
        listing.api.queues().catch(() => []),
      ]);
      have = await listing.api.linksHave(listing.id).catch(() => []);
      picks = startingPicks(v.links, have);
      folder = v.folder;
      places = folders;
      home = homeDir;
      queues = queueList;
      // Pasted links are all wanted, web pages or not; a page's pages rarely are.
      pages = v.source === "pasted" || v.links.every((l) => l.group === "pages");
      view = v;
      void lookAtAll(v.links);
    } catch (error) {
      failure = messageOf(error);
    }
  }
  void load();

  // Probe the links a handful at a time, visible rows first.
  async function lookAtAll(list: Link[]) {
    const shown = new Set(order);
    const queue = [...list.keys()]
      .filter((i) => list[i]?.group !== "videoPages" && !list[i]?.looked)
      .sort((a, b) => Number(shown.has(b)) - Number(shown.has(a)));
    for (let at = 0; at < queue.length; at += 16) {
      const batch = queue.slice(at, at + 16);
      const looked = await listing.api.lookLinks(listing.id, batch).catch(() => null);
      if (!looked || !view) return;
      const next = [...view.links];
      batch.forEach((i, n) => {
        const l = looked[n];
        if (!l) return;
        // A link that turned out to be a file people download is ticked once it is known.
        const before = next[i];
        if (before && !isFile(before) && isFile(l) && !have[i]) picks[i] = true;
        next[i] = l;
      });
      view = { ...view, links: next };
    }
  }

  function toggleGroup(entries: number[]) {
    const on = !entries.every((i) => picks[i]);
    const these = new Set(entries);
    picks = picks.map((p, i) => (these.has(i) ? on : p));
  }

  function setShown(on: boolean) {
    const these = new Set(order);
    picks = picks.map((p, i) => (these.has(i) ? on : p));
  }

  async function download() {
    if (!view || busy || !chosen.length) return;
    busy = true;
    failure = "";
    try {
      await listing.api.downloadLinks(listing.id, {
        entries: picked(picks).filter((i) => chosen.includes(i)),
        directory: folder.trim(),
        queue,
        keepFolders: hasFolders && keepFolders,
      });
    } catch (error) {
      failure = messageOf(error);
    } finally {
      busy = false;
    }
  }

  async function pickFolder() {
    picking = true;
    try {
      const chosenFolder = await listing.api.chooseFolder(folder);
      if (chosenFolder) folder = chosenFolder;
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

<main aria-label="Download links">
  {#if view}
    <div class="body">
      <div class="what">
        <span class="tile"><Icon name="link" size={19} /></span>
        <div class="texts">
          <h1>{view.title || "Links"}</h1>
          <div class="from">{about}</div>
        </div>
      </div>
      {#if view.note}<p class="how">{view.note}</p>{/if}

      <div class="search field">
        <Icon name="search" size={15} />
        <input
          aria-label="Filter"
          placeholder="Filter: words, or *.pdf"
          bind:value={filter}
          disabled={busy}
          spellcheck="false"
        />
      </div>
      <div class="toggles">
        {#if hasPages}
          <label class="check"
            ><input type="checkbox" bind:checked={pages} disabled={busy} />Web pages</label
          >
        {/if}
        {#if hasOtherSites}
          <label class="check"
            ><input type="checkbox" bind:checked={sameSite} disabled={busy} />Only from {pageHost}</label
          >
        {/if}
        {#if have.some(Boolean)}
          <label class="check"
            ><input type="checkbox" bind:checked={hideHave} disabled={busy} />Hide what Snag has</label
          >
        {/if}
      </div>

      <div class="choose">
        <div class="places">
          <button disabled={busy} onclick={() => setShown(true)}>All</button>
          <button disabled={busy} onclick={() => setShown(false)}>None</button>
          {#if groups.length > 1}
            <span class="divider"></span>
            {#each groups as g (g.id)}
              {@const all = g.entries.every((i) => picks[i])}
              <button
                aria-pressed={all}
                class:chosen={all}
                class:some={!all && g.entries.some((i) => picks[i])}
                disabled={busy}
                onclick={() => toggleGroup(g.entries)}>{g.name} {g.entries.length}</button
              >
            {/each}
          {/if}
        </div>
        <span class="count">{chosen.length} of {order.length}</span>
      </div>

      <TickList
        bind:picks
        {order}
        {busy}
        name="Links"
        label={(i) => view?.links[i]?.server || view?.links[i]?.name || ""}
      >
        {#snippet row(i: number)}
          {@const l = view!.links[i]!}
          {@const other = pageHost && host(l.url) !== pageHost ? host(l.url) : ""}
          <span class="name" title={l.url}
            >{#if l.dir}<span class="dir">{l.dir}/</span>{/if}{l.server || l.name}</span
          >
          {#if l.text && l.text !== l.name && l.text !== l.server}<span class="was">{l.text}</span
            >{/if}
          {#if other}<span class="mark">{other}</span>{/if}
          {#if have[i]}<span class="mark">In Snag</span>{/if}
          <span class="kind">{groupNames[l.group] ?? ""}</span>
          <span class="length">{l.size ? size(l.size) : ""}</span>
        {/snippet}
      </TickList>
      {#if order.length === 0}
        <p class="how">Nothing matches. Clear the filter, or show web pages.</p>
      {/if}

      <label class="caption" for="links-folder">Save to</label>
      <div class="folder-row">
        <div class="field">
          <Icon name="folder" size={16} />
          <input id="links-folder" bind:value={folder} disabled={busy} />
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
          <button class:chosen={folder === place} disabled={busy} onclick={() => (folder = place)}
            >{short(place)}</button
          >
        {/each}
      </div>
      {#if hasFolders}
        <div class="toggles">
          <label class="check"
            ><input type="checkbox" bind:checked={keepFolders} disabled={busy} />Keep the site's
            folders</label
          >
        </div>
      {/if}

      {#if queues.length > 1}
        <div class="caption">Queue</div>
        <div class="places" role="radiogroup" aria-label="Queue">
          {#each queues as q (q.id)}
            <button
              role="radio"
              aria-checked={queue === q.id}
              class:chosen={queue === q.id}
              disabled={busy}
              onclick={() => (queue = q.id)}>{q.name}</button
            >
          {/each}
        </div>
      {/if}

      {#if failure}<p class="failure" role="alert">{failure}</p>{/if}
    </div>

    <div class="buttons">
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
        label={`Download ${chosen.length}`}
        kind="primary"
        size={38}
        tip={chosen.length
          ? total
            ? `${size(total)} known so far`
            : "Add them to the queue"
          : "Tick some first"}
        {busy}
        disabled={busy || !chosen.length || !folder.trim()}
        onclick={download}
      />
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

  .search {
    flex: none;
    margin-top: 14px;
  }

  .toggles {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
    margin-top: 10px;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--text-dim);
    font-size: var(--text-sm);
    cursor: pointer;
  }

  .check input {
    flex: none;
    margin: 0;
  }

  .choose {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-top: 8px;
  }

  .count {
    flex: none;
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

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dir {
    color: var(--text-faint);
  }

  .was {
    flex: 0 1 auto;
    max-width: 34%;
    min-width: 0;
    overflow: hidden;
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mark {
    flex: none;
    max-width: 30%;
    overflow: hidden;
    padding: 1px 7px;
    border-radius: 9px;
    background: var(--raised);
    color: var(--text-dim);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kind {
    flex: none;
    width: 6.5em;
    color: var(--text-faint);
    font-size: var(--text-sm);
    text-align: right;
  }

  .length {
    flex: none;
    width: 5em;
    color: var(--text-faint);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    text-align: right;
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

  .search.field {
    flex: none;
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

  .how {
    margin: 10px 0 0;
    color: var(--text-dim);
    font-size: var(--text-sm);
    line-height: 1.45;
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
    margin-top: 12px;
  }
</style>
