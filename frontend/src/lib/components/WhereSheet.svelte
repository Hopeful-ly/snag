<script lang="ts">
  import IconButton from "./IconButton.svelte";
  import Sheet from "./Sheet.svelte";
  import type { Found } from "../api/client";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const d = $derived(snag.whereFor ? snag.downloads.get(snag.whereFor) : undefined);

  const places = [
    { id: "youtube", label: "YouTube" },
    { id: "soundcloud", label: "SoundCloud" },
  ];

  let elsewhere = $state(false);
  let place = $state("");
  let results = $state<Found[] | null>(null);
  let picked = $state("");
  let link = $state("");
  let remember = $state(false);
  let looking = $state(false);

  const placeName = $derived(places.find((p) => p.id === place)?.label ?? "");
  const using = $derived(!!d && !!snag.busy[`${d.id}:where`]);
  const retrying = $derived(!!d && !!snag.busy[`${d.id}:resume`]);
  const canUse = $derived(
    elsewhere && (picked !== "" || link.trim() !== "") && !looking && !using && !retrying,
  );

  function close() {
    snag.whereFor = null;
  }

  function length(seconds: number): string {
    if (!seconds) return "";
    const m = Math.floor(seconds / 60);
    return `${m}:${String(Math.round(seconds % 60)).padStart(2, "0")}`;
  }

  async function retryMusic() {
    const song = d;
    if (!song || retrying || using) return;
    const ok = await snag.request(`${song.id}:resume`, () => snag.api.resume(song.id));
    if (ok) close();
  }

  async function look(next: string) {
    const song = d;
    if (!song || looking || using) return;
    place = next;
    results = null;
    picked = "";
    remember = false;
    looking = true;
    let found: Found[] | undefined;
    const ok = await snag.request(`${song.id}:search`, async () => {
      found = await snag.api.searchElsewhere(song.id, next);
    });
    looking = false;
    if (!ok || !found) return;
    results = found;
    picked = found.find((r) => r.match)?.url ?? "";
  }

  async function use() {
    const song = d;
    if (!song || !canUse) return;
    const chosen = picked || link.trim();
    const forPlace = picked ? place : "";
    const ok = await snag.request(`${song.id}:where`, () =>
      snag.api.useFound(song.id, chosen, forPlace, remember && forPlace !== ""),
    );
    if (ok) close();
  }
</script>

{#if d}
  <Sheet title="Try again" icon="retry" width={520} locked={using || retrying} onclose={close}>
    <p class="lead">YouTube Music has no match for “{d.name}”.</p>

    <div class="choices" role="group" aria-label="Try again">
      <button type="button" disabled={looking || using || retrying} onclick={retryMusic}
        >Retry YouTube Music</button
      >
      <button
        type="button"
        class:on={elsewhere}
        aria-pressed={elsewhere}
        disabled={looking || using || retrying}
        onclick={() => (elsewhere = true)}>Another source</button
      >
    </div>

    {#if elsewhere}
      <div class="choices places" role="group" aria-label="Look on">
        {#each places as p (p.id)}
          <button
            type="button"
            class:on={place === p.id}
            aria-pressed={place === p.id}
            disabled={looking || using}
            onclick={() => look(p.id)}>{p.label}</button
          >
        {/each}
      </div>

      {#if looking}
        <p class="note">Looking on {placeName}…</p>
      {:else if results}
        {#if results.length === 0}
          <p class="note">Nothing came up on {placeName}. Try the other place, or paste a link.</p>
        {:else}
          <div class="results" role="radiogroup" aria-label="On {placeName}">
            {#each results as r (r.url)}
              <button
                type="button"
                role="radio"
                aria-checked={picked === r.url}
                class:on={picked === r.url}
                disabled={using}
                onclick={() => (picked = r.url)}
              >
                <span class="title">{r.title}</span>
                <span class="by">
                  {r.by}{#if r.seconds}&nbsp;· {length(r.seconds)}{/if}{#if r.match}&nbsp;· Matches{/if}
                </span>
              </button>
            {/each}
          </div>
        {/if}
        <label class="remember">
          <input type="checkbox" bind:checked={remember} disabled={using} />
          Look on {placeName} for the other songs YouTube Music doesn't have
        </label>
      {/if}

      <label class="caption" for="where-link">Or paste a link</label>
      <input
        id="where-link"
        class="field"
        placeholder="https://"
        bind:value={link}
        disabled={using}
        oninput={() => (picked = "")}
        onkeydown={(e) => {
          if (e.key === "Enter") {
            picked = "";
            void use();
          }
        }}
      />

      <div class="buttons">
        <span class="spacer"></span>
        <IconButton
          icon="download"
          label="Use this"
          kind="primary"
          busy={using}
          disabled={!canUse}
          onclick={use}
        />
      </div>
    {/if}
  </Sheet>
{/if}

<style>
  .lead {
    margin: 0 0 14px;
  }

  .choices {
    display: flex;
    gap: 8px;
  }

  .places {
    margin-top: 8px;
  }

  .choices button,
  .results button {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--field);
    color: var(--text);
    text-align: left;
  }

  .choices button {
    flex: 1;
    height: 36px;
    font: inherit;
  }

  .choices button.on,
  .results button.on {
    border-color: var(--accent-strong);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .results {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 240px;
    margin-top: 12px;
    overflow: auto;
  }

  .results button {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 8px 10px;
    font: inherit;
    cursor: pointer;
  }

  .title {
    font-weight: 600;
  }

  .by {
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .results button.on .by {
    color: var(--accent);
  }

  .note,
  .remember {
    margin: 12px 0 0;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .remember {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .caption {
    display: block;
    margin: 16px 0 6px;
    color: var(--text-dim);
    font-size: var(--text-sm);
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

  .buttons {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 16px;
  }

  .spacer {
    flex: 1;
  }
</style>
